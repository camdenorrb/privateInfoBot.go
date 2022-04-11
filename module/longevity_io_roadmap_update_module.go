package module

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
	"github.com/json-iterator/go"
	"github.com/pkg/errors"
	"golang.org/x/net/html/atom"
	"log"
	"net/http"
	"os"
	"path"
	"privateInfoBot/utils"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const roadmapURL = "https://www.lifespan.io/road-maps/the-rejuvenation-roadmap/"

type LongevityChangeLogEntry struct {
	Date    time.Time `json:"date"`
	Updates []string  `json:"updates"`
}

type LongevityIORoadmapUpdateModule struct {
	isEnabled  bool
	checkDelay time.Duration
	channelID  uint64
	lastItems  []*LongevityChangeLogEntry
	discord    *discordgo.Session
}

func NewLongevityIORoadmapUpdateModule(
	checkDelay time.Duration,
	channelID uint64,
	discord *discordgo.Session,
) *LongevityIORoadmapUpdateModule {
	return &LongevityIORoadmapUpdateModule{
		checkDelay: checkDelay,
		channelID:  channelID,
		discord:    discord,
	}
}

func (module *LongevityIORoadmapUpdateModule) IsEnabled() bool {
	return module.isEnabled
}

func (module *LongevityIORoadmapUpdateModule) Enable() {
	if !module.isEnabled {
		module.isEnabled = true
		module.lastItems = module.pullSavedData()
		go module.updateTask(len(module.lastItems) == 0)
	}
}

func (module *LongevityIORoadmapUpdateModule) Disable() {
	module.isEnabled = false
}

// pullSavedData Pulls data from the saved file from the last update
func (module *LongevityIORoadmapUpdateModule) pullSavedData() []*LongevityChangeLogEntry {

	jsonData, err := os.ReadFile(module.filePath())
	if err != nil {

		if errors.Is(err, os.ErrNotExist) {
			return []*LongevityChangeLogEntry{}
		}

		log.Fatal(errors.Wrap(err, "pullSavedData error"))
	}

	var result = new([]*LongevityChangeLogEntry)

	err = jsoniter.Unmarshal(jsonData, result)
	if err != nil {
		log.Fatal(errors.Wrap(err, "pullSavedData error"))
	}

	return *result
}

func (module *LongevityIORoadmapUpdateModule) updateTask(skipPostingFirstRun bool) {

	hasSkipped := false

	for module.isEnabled {

		pulledItems, err := module.pullItems()
		if err != nil {
			log.Printf("%v", errors.Wrapf(err, "failed to pull items for longevity io roadmap"))
			time.Sleep(module.checkDelay)
			continue
		}

		if !hasSkipped && skipPostingFirstRun {
			hasSkipped = true
			module.lastItems = pulledItems
			module.saveLastItems()
			time.Sleep(module.checkDelay)
			continue
		}

		recentUpdates := module.difference(module.lastItems, pulledItems)
		if len(recentUpdates) == 0 {
			module.postUpdates(recentUpdates)
		}

		module.lastItems = pulledItems
		module.saveLastItems()

		time.Sleep(module.checkDelay)
	}
}

func (module *LongevityIORoadmapUpdateModule) postUpdates(items []*LongevityChangeLogEntry) {

	messagesToSend := module.itemsToMessages(items)

	for _, messageToSend := range messagesToSend {

		channelIDString := strconv.FormatUint(module.channelID, 10)

		message, err := module.discord.ChannelMessageSendComplex(channelIDString, &messageToSend)
		if err != nil {
			log.Fatal(fmt.Errorf("postUpdates failed channel (%s): %w", channelIDString, err))
		}

		_, err = module.discord.ChannelMessageCrosspost(channelIDString, message.ID)
		if err != nil {
			log.Fatal(fmt.Errorf("postUpdates failed channel (%s): %w", channelIDString, err))
		}
	}
}

func (module *LongevityIORoadmapUpdateModule) itemsToMessages(items []*LongevityChangeLogEntry) (messages []discordgo.MessageSend) {

	hexColor, err := strconv.ParseUint("e1ad01", 16, 32)
	if err != nil {
		log.Fatal(errors.Wrap(err, "itemsToRedditMessages failed"))
	}

	for _, item := range items {

		embed := &discordgo.MessageEmbed{
			URL:         roadmapURL,
			Title:       fmt.Sprintf("Update found to %v!", item.Date.Format("2 January 2006")),
			Description: "https://www.lifespan.io/road-maps/the-rejuvenation-roadmap/",
			Timestamp:   item.Date.String(),
			Color:       int(hexColor),
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: "https://pbs.twimg.com/profile_images/1303743628569968642/CAUc2pVY_400x400.jpg",
			},
		}

		field := discordgo.MessageEmbedField{
			Name:  "Changelog",
			Value: "\n",
		}

		for i, update := range item.Updates {

			if i > 0 {
				field.Value += "\n\n"
			}

			title := strings.TrimFunc(update, func(r rune) bool {
				return r == ' ' || r == '\n'
			})

			field.Value += ":green_circle: " + title
		}

		embed.Fields = []*discordgo.MessageEmbedField{&field}
		messages = append(messages, discordgo.MessageSend{Embed: embed})
	}

	return
}

func (module *LongevityIORoadmapUpdateModule) difference(oldValues []*LongevityChangeLogEntry, newValues []*LongevityChangeLogEntry) []*LongevityChangeLogEntry {

	var result []*LongevityChangeLogEntry

	for _, newValue := range newValues {

		isOld := false

		for _, oldValue := range oldValues {
			if reflect.DeepEqual(oldValue, newValue) {
				isOld = true
				break
			}
		}

		if !isOld {
			result = append(result, newValue)
		}
	}

	return result
}

func (module *LongevityIORoadmapUpdateModule) pullItems() ([]*LongevityChangeLogEntry, error) {

	fmt.Printf("(%v) Pulling: %v\n", time.Now().Format("02 Jan 2006 03:04PM MST"), roadmapURL)

	client := new(http.Client)

	request, err := http.NewRequest("GET", roadmapURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "pullUpdates error")
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "pullUpdates error")
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get document reader")
	}

	var lastDate *time.Time
	var changeLogEntries []*LongevityChangeLogEntry
	doc.Find("#content > div:nth-child(6)").Children().EachWithBreak(func(i int, selection *goquery.Selection) bool {

		switch selection.Get(0).DataAtom {

		case atom.P:

			// Needs to be defined this way to not redefine `err`
			lastDate, err = parseDate(strings.TrimSpace(selection.Text()))
			if err != nil {
				err = errors.WithStack(err)
				return false
			}

		case atom.Ul:

			if lastDate == nil {
				err = errors.Errorf("lastDate is nil for: %s", selection.Text())
				return false
			}

			var updates []string
			for _, update := range strings.Split(strings.Trim(selection.Text(), "\n"), "\n") {
				updates = append(updates, update)
			}

			entry := LongevityChangeLogEntry{
				Date:    *lastDate,
				Updates: updates,
			}

			changeLogEntries = append(changeLogEntries, &entry)

		case atom.Section:
			// Skip section (Can parse in the future, but is only 2 changelogs)
			return true

		default:
			fmt.Println(selection.Text())
			err = errors.Errorf("unexpected tag data: %s", selection.Get(0).Data)

		}

		return err == nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "pullUpdates error")
	}

	err = response.Body.Close()
	if err != nil {
		return nil, errors.Wrap(err, "pullUpdates error")
	}

	return changeLogEntries, nil
}

func (module *LongevityIORoadmapUpdateModule) saveLastItems() {
	err := utils.WriteJsonAfterMakeDirs(module.filePath(), module.lastItems)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to saveLastItems"))
	}
}

func (module *LongevityIORoadmapUpdateModule) filePath() string {
	return path.Join("Modules", "LongevityIORoadmap", "longevity_io_roadmap.json")
}

func parseDate(input string) (*time.Time, error) {

	lastDateParsed, err := time.Parse("02 January 2006", input)
	if err == nil {
		return &lastDateParsed, nil
	}

	lastDateParsed, err = time.Parse("2 January 2006", input)
	if err == nil {
		return &lastDateParsed, nil
	}

	lastDateParsed, err = time.Parse("02 Jan 2006", input)
	if err == nil {
		return &lastDateParsed, nil
	}

	lastDateParsed, err = time.Parse("2 Jan 2006", input)
	if err == nil {
		return &lastDateParsed, nil
	}

	return nil, errors.Errorf("failed to parse date: %s", input)
}
