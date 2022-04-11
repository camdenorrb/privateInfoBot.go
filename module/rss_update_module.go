package module

import (
	"crypto/tls"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
	"github.com/json-iterator/go"
	"github.com/mmcdole/gofeed"
	"github.com/pkg/errors"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"privateInfoBot/data"
	"privateInfoBot/utils"
	"strconv"
	"strings"
	"time"
)

type RSSUpdateModule struct {
	isEnabled  bool
	checkDelay time.Duration
	rssFeed    data.RSSFeed
	channels   map[string]uint64
	lastItems  []*gofeed.Item
	discord    *discordgo.Session
}

func NewRSSUpdateModule(
	checkDelay time.Duration,
	rssFeed data.RSSFeed,
	channels map[string]uint64,
	discord *discordgo.Session,
) *RSSUpdateModule {
	return &RSSUpdateModule{
		checkDelay: checkDelay,
		rssFeed:    rssFeed,
		channels:   channels,
		discord:    discord,
	}
}

func (module *RSSUpdateModule) IsEnabled() bool {
	return module.isEnabled
}

func (module *RSSUpdateModule) Enable() {
	if !module.isEnabled {
		module.isEnabled = true
		module.lastItems = module.pullSavedData()
		go module.updateTask(len(module.lastItems) == 0)
	}
}

func (module *RSSUpdateModule) Disable() {
	module.isEnabled = false
}

func (module *RSSUpdateModule) updateTask(skipPostingFirstRun bool) {

	hasSkipped := false

	for module.isEnabled {

		pulledItems, err := module.pullItems()
		if err != nil {
			log.Printf("%v", fmt.Errorf("failed to pull items for %v: %w", module.rssFeed.FeedURL, err))
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

		recentUpdates := module.filterRecentUpdates(pulledItems)
		if recentUpdates != nil {
			module.postUpdates(recentUpdates)
		}

		module.lastItems = pulledItems
		module.saveLastItems()

		time.Sleep(module.checkDelay)
	}
}

func (module *RSSUpdateModule) postUpdates(items []*gofeed.Item) {

	var messagesToSend []discordgo.MessageSend

	if module.rssFeed.Type != nil {

		switch *module.rssFeed.Type {

		case data.Reddit:
			messagesToSend = module.itemsToRedditMessages(items)
		case data.Github:
			messagesToSend = module.itemsToGithubMessages(items)
		case data.TitleAndLink:
			messagesToSend = module.itemsToTitleAndLinkMessages(items)
		case data.KernelOrgUpdates:
			messagesToSend = module.itemsToKernelOrgMessages(items)

		default:
			messagesToSend = module.itemsToDefaultMessages(items)

		}

	}

	channelID := module.channels[module.rssFeed.ChannelName]

	for _, messageToSend := range messagesToSend {

		channelIDString := strconv.FormatUint(channelID, 10)

		message, err := module.discord.ChannelMessageSendComplex(channelIDString, &messageToSend)
		if err != nil {
			log.Fatal(fmt.Errorf("postUpdates failed channel (%s:%s): %w", module.rssFeed.ChannelName, channelIDString, err))
		}

		_, err = module.discord.ChannelMessageCrosspost(channelIDString, message.ID)
		if err != nil {
			log.Fatal(fmt.Errorf("postUpdates failed channel (%s:%s): %w", module.rssFeed.ChannelName, channelIDString, err))
		}
	}
}

func (module *RSSUpdateModule) itemsToRedditMessages(items []*gofeed.Item) (messages []discordgo.MessageSend) {

	for _, item := range items {

		embed := module.embedTemplateForItem(item)

		document, err := goquery.NewDocumentFromReader(strings.NewReader(item.Content))
		if err != nil {
			log.Fatal(fmt.Errorf("itemsToRedditMessages failed: %w", err))
		}

		hyperLink, _ := document.Find("td a").First().Attr("href")
		imageLink, _ := document.Find("td img").First().Attr("src")

		if hyperLink != "" {
			embed.Description = hyperLink
		}
		if embed.Image != nil {
			embed.Image = &discordgo.MessageEmbedImage{URL: imageLink}
		}

		messages = append(messages, discordgo.MessageSend{Embed: embed})
	}

	return
}

func (module *RSSUpdateModule) itemsToGithubMessages(items []*gofeed.Item) (messages []discordgo.MessageSend) {

	embed := &discordgo.MessageEmbed{
		URL: strings.TrimSuffix(module.rssFeed.FeedURL, "/commits/master.atom"),
	}

	module.applyTitle(embed)
	module.applyDescription(embed)
	module.applyColor(embed)
	module.applyThumbnail(embed)

	field := discordgo.MessageEmbedField{
		Name:  "New commit messages",
		Value: "\n",
	}

	for i, item := range items {

		if i > 0 {
			field.Value += "\n\n"
		}

		title := strings.TrimFunc(item.Title, func(r rune) bool {
			return r == ' ' || r == '\n'
		})

		field.Value += ":green_circle: " + title
	}

	embed.Fields = []*discordgo.MessageEmbedField{&field}
	return []discordgo.MessageSend{{Embed: embed}}
}

func (module *RSSUpdateModule) itemsToTitleAndLinkMessages(items []*gofeed.Item) (messages []discordgo.MessageSend) {

	for _, item := range items {
		messages = append(messages,
			discordgo.MessageSend{
				Content: fmt.Sprintf("**%v**\n%v", item.Title, item.Link),
			},
		)
	}

	return
}

func (module *RSSUpdateModule) itemsToKernelOrgMessages(items []*gofeed.Item) (messages []discordgo.MessageSend) {

	embed := &discordgo.MessageEmbed{
		URL: module.rssFeed.FeedURL,
	}

	module.applyTitle(embed)
	module.applyDescription(embed)
	module.applyColor(embed)
	module.applyThumbnail(embed)

	field := discordgo.MessageEmbedField{
		Name:  "New versions",
		Value: "\n",
	}

	for i, item := range items {

		if i > 0 {
			field.Value += "\n\n"
		}

		title := strings.TrimFunc(item.Title, func(r rune) bool {
			return r == ' ' || r == '\n'
		})

		field.Value += ":green_circle: " + title
	}

	embed.Fields = []*discordgo.MessageEmbedField{&field}
	return []discordgo.MessageSend{{Embed: embed}}
}

func (module *RSSUpdateModule) itemsToDefaultMessages(items []*gofeed.Item) (messages []discordgo.MessageSend) {

	for _, item := range items {
		messages = append(messages,
			discordgo.MessageSend{
				Embed: &discordgo.MessageEmbed{
					Description: html.UnescapeString(item.Description),
				},
			})
	}

	return
}

func (module *RSSUpdateModule) pullItems() ([]*gofeed.Item, error) {

	fmt.Printf("(%v) Pulling: %v\n", time.Now().Format("02 Jan 2006 03:04PM MST"), module.rssFeed.FeedURL)

	client := new(http.Client)

	request, err := http.NewRequest("GET", module.rssFeed.FeedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("pullUpdates error: %w", err)
	}

	isReddit := module.rssFeed.Type != nil && *module.rssFeed.Type == data.Reddit

	// Fix weird Reddit issue "Status Code: 429" https://www.reddit.com/r/redditdev/comments/t8e8hc/getting_nothing_but_429_responses_when_using_go/
	if isReddit {
		client.Transport = &http.Transport{
			TLSNextProto: map[string]func(authority string, c *tls.Conn) http.RoundTripper{},
		}
		request.Header.Set("User-Agent", "Mozilla/5.0")
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("pullUpdates error: %w", err)
	}

	var rssFeed *gofeed.Feed

	// Repair content type if it is Reddit
	if isReddit {

		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("pullUpdates error: %w", err)
		}

		bodyText := strings.Replace(string(bodyBytes), "text/html", "application/rss+xml", 1)

		rssFeed, err = gofeed.NewParser().ParseString(bodyText)
		if err != nil {
			return nil, fmt.Errorf("pullUpdates error: %w", err)
		}
	} else {
		rssFeed, err = gofeed.NewParser().Parse(response.Body)
		if err != nil {
			return nil, fmt.Errorf("pullUpdates error: %w", err)
		}
	}

	err = response.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("pullUpdates error: %w", err)
	}

	return rssFeed.Items, nil
}

func (module *RSSUpdateModule) filterRecentUpdates(items []*gofeed.Item) (updates []*gofeed.Item) {

	for _, item := range module.difference(module.lastItems, items) {

		timeSincePublished := time.Second * 1
		timeSinceUpdated := time.Second * 1

		if item.PublishedParsed != nil {
			timeSincePublished = time.Since(*item.PublishedParsed)
		}

		if item.UpdatedParsed != nil {
			timeSinceUpdated = time.Since(*item.UpdatedParsed)
		}

		if timeSincePublished.Hours() < 24 && timeSinceUpdated.Hours() < 24 {
			itemCopy := item // needed since item changes what it points to
			updates = append(updates, itemCopy)
		}
	}

	return
}

func (module *RSSUpdateModule) difference(oldValues []*gofeed.Item, newValues []*gofeed.Item) []*gofeed.Item {

	var result []*gofeed.Item

	for _, newValue := range newValues {

		isOld := false

		for _, oldValue := range oldValues {
			if oldValue.Title == newValue.Title {
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

func (module *RSSUpdateModule) filePath() string {

	var fileName string

	if module.rssFeed.FileName != nil {
		fileName = *module.rssFeed.FileName
	} else {
		fileName = utils.SubstringAfter(module.rssFeed.FeedURL, "//")
		fileName = utils.SubstringAfter(fileName, "www.")
		fileName = strings.ReplaceAll(fileName, "/", "_")
	}

	return path.Join("Modules", "RSS", fileName+".json")
}

func (module *RSSUpdateModule) saveLastItems() {
	err := utils.WriteJsonAfterMakeDirs(module.filePath(), module.lastItems)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to saveLastItems"))
	}
}

// pullSavedData Pulls data from the saved file from the last update
func (module *RSSUpdateModule) pullSavedData() []*gofeed.Item {

	jsonData, err := os.ReadFile(module.filePath())
	if err != nil {

		if errors.Is(err, os.ErrNotExist) {
			return []*gofeed.Item{}
		}

		log.Fatal(fmt.Errorf("pullSavedData error: %w\n", err))
	}

	var result = new([]*gofeed.Item)

	err = jsoniter.Unmarshal(jsonData, result)
	if err != nil {
		log.Fatal(fmt.Errorf("pullSavedData error: %w\n", err))
	}

	return *result
}

func (module *RSSUpdateModule) applyTitle(embed *discordgo.MessageEmbed) {
	if module.rssFeed.Title != nil {
		embed.Title = *module.rssFeed.Title
	}
}

func (module *RSSUpdateModule) applyColor(embed *discordgo.MessageEmbed) {
	if module.rssFeed.Color != nil {

		color, err := strconv.ParseUint(strings.TrimPrefix(*module.rssFeed.Color, "#"), 16, 32)
		if err != nil {
			log.Fatal(fmt.Errorf("applyColor failed: %w", err))
		}

		embed.Color = int(color)
	}
}

func (module *RSSUpdateModule) applyAuthor(item *gofeed.Item, embed *discordgo.MessageEmbed) {
	if module.rssFeed.Author != nil {

		author := *module.rssFeed.Author
		if len(item.Authors) > 0 {
			author = strings.ReplaceAll(author, "${entryAuthor}", item.Authors[0].Name)
		}

		embed.Author = &discordgo.MessageEmbedAuthor{Name: author}
	}
}

func (module *RSSUpdateModule) applyThumbnail(embed *discordgo.MessageEmbed) {
	if module.rssFeed.ThumbnailURL != nil {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: *module.rssFeed.ThumbnailURL}
	}
}

func (module *RSSUpdateModule) applyDescription(embed *discordgo.MessageEmbed) {
	if module.rssFeed.Description != nil {
		embed.Description = *module.rssFeed.Description
	}
}

func (module *RSSUpdateModule) embedTemplateForItem(item *gofeed.Item) *discordgo.MessageEmbed {

	embed := &discordgo.MessageEmbed{
		URL:       item.Link,
		Title:     item.Title,
		Timestamp: item.Published,
	}

	module.applyColor(embed)
	module.applyAuthor(item, embed)
	module.applyThumbnail(embed)
	module.applyDescription(embed)

	return embed
}
