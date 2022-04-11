package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	jsoniter "github.com/json-iterator/go"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"privateInfoBot/data"
	"privateInfoBot/module"
	"syscall"
	"time"
)

// TODO: Mess with these: https://discord.com/developers/docs/interactions/message-components
func main() {

	rssFeedsJson, err := os.ReadFile("rssFeeds.json")
	if err != nil {
		log.Fatal(fmt.Errorf("failed to read rssFeeds: %w", err))
	}

	rssFeeds := new([]data.RSSFeed)

	err = jsoniter.Unmarshal(rssFeedsJson, rssFeeds)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to read rssFeeds: %w", err))
	}

	channelsJson, err := os.ReadFile("channels.json")
	if err != nil {
		log.Fatal(fmt.Errorf("failed to read channels: %w", err))
	}

	channels := new(map[string]uint64)

	err = jsoniter.Unmarshal(channelsJson, channels)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to read channels: %w", err))
	}

	token, err := ioutil.ReadFile("token.txt")
	if err != nil {
		log.Fatal(fmt.Errorf("failed to read token: %w", err))
	}

	discord, err := discordgo.New("Bot " + string(token))
	discord.Identify.Intents = discordgo.IntentsGuildMessages

	err = discord.Open()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to start discord bot: %w", err))
	}

	var rssModules []module.RSSUpdateModule
	for _, feed := range *rssFeeds {
		rssModule := module.NewRSSUpdateModule(time.Minute*30, feed, *channels, discord)
		rssModules = append(rssModules, *rssModule)
		rssModule.Enable()
	}

	module.NewLongevityIORoadmapUpdateModule(
		time.Minute*30,
		(*channels)["longevityNews"],
		discord,
	).Enable()

	time.Sleep(time.Second * 2)

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}
