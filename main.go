package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sardap/discom"
)

var (
	commandSet    *discom.CommandSet
	targetGuild   string
	targetChannel string
)

func init() {
	rand.Seed(time.Now().UnixMicro())

	targetGuild = os.Getenv("TARGET_GUILD")
	targetChannel = os.Getenv("TARGET_CHANNEL")

	commandSet, _ = discom.CreateCommandSet("rwb", errorHandler)

	err := commandSet.AddCommand(discom.Command{
		Name: "what", Handler: helpCommand,
		Description: "Explains what I do.",
	})
	if err != nil {
		panic(err)
	}

}

func errorHandler(s *discordgo.Session, i discom.Interaction, err error) {

}

func helpCommand(s *discordgo.Session, i discom.Interaction) error {
	i.Respond(s,
		discom.Response{
			Content: "Do you know when google photos goes 2 years ago your dog died here" +
				" are some photos of them when they were at there sickest." +
				" This is like that with discord messages",
		},
	)
	return nil
}

type MessageArchive struct {
	Timestamp time.Time
	Id        string
}

type MessageCache struct {
	Channels map[string][]MessageArchive
}

func loadCache() MessageCache {
	var cache MessageCache
	data, _ := os.ReadFile("cache.json")
	json.Unmarshal(data, &cache)
	if cache.Channels == nil {
		cache.Channels = map[string][]MessageArchive{}
	}

	return cache
}

func createCache(s *discordgo.Session) {
	fmt.Printf("Starting cache update\n")

	cache := loadCache()

	fmt.Printf("Old cache loaded\n")

	fmt.Printf("Pulling channels\n")
	channels, _ := s.GuildChannels(targetGuild)
	for _, ch := range channels {
		fmt.Printf("Processing channel %s\n", ch.ID)
		top := ""
		if messages, ok := cache.Channels[ch.ID]; ok {
			sort.Slice(messages, func(i, j int) bool {
				return messages[i].Timestamp.After(messages[j].Timestamp)
			})
			top = messages[0].Id
		}
		for {
			fmt.Printf("for channel %s Processing after %s\n", ch.ID, top)
			messages, _ := s.ChannelMessages(ch.ID, 100, "", top, "")
			if len(messages) == 0 {
				break
			}
			for _, message := range messages {
				cache.Channels[ch.ID] = append(
					cache.Channels[ch.ID],
					MessageArchive{
						Timestamp: message.Timestamp,
						Id:        message.ID,
					},
				)
			}
			top = messages[len(messages)-1].ID
		}
	}

	fmt.Printf("Done updating cache writing to file\n")
	{
		data, _ := json.Marshal(cache)
		os.WriteFile("cache.json", data, 0777)
	}
	fmt.Printf("Done Writing cache to file\n")
}

type MessageArchiveComplete struct {
	MessageArchive
	ChannelId string
}

func getAllMessages(cutoffTime time.Time) (result []MessageArchiveComplete) {
	cache := loadCache()
	for chId, ch := range cache.Channels {
		for _, message := range ch {
			if message.Timestamp.Before(cutoffTime) {
				result = append(result, MessageArchiveComplete{MessageArchive: message, ChannelId: chId})
			}
		}
	}
	return
}

func rememberWhen(s *discordgo.Session) {
	messages := getAllMessages(time.Now().Add(-(365 * 24 * time.Hour)))

	targetMessage := messages[rand.Intn(len(messages)-1)]

	message, err := s.ChannelMessage(targetMessage.ChannelId, targetMessage.Id)
	if err != nil || message.Content == "" || len(message.Attachments) > 0 {
		// Fuck it
		rememberWhen(s)
		return
	}

	tz, err := time.LoadLocation("Australia/Melbourne")
	if err != nil {
		panic(err)
	}

	channel, _ := s.Channel(message.ChannelID)

	content := fmt.Sprintf(
		"Remember when <@%s> said \"%s\" on %s at %s in %s\n",
		message.Author, strings.ReplaceAll(message.Content, "\n", " "),
		message.Timestamp.In(tz).Format("Mon, 02 Jan 2006"),
		message.Timestamp.In(tz).Format("15:04:05"),
		strings.ReplaceAll(channel.Name, "\n", ""),
	)

	fmt.Printf("Will send:%s", content)
	s.ChannelMessageSend(targetChannel, content)
}

func rememberWhenWorker(s *discordgo.Session) {
	nextTimeFileName := "next_time.txt"
	for {
		data, _ := os.ReadFile(nextTimeFileName)
		if len(data) > 0 {
			sleepUntil, _ := time.Parse(time.UnixDate, string(data))
			time.Sleep(time.Until(sleepUntil))
		}

		createCache(s)
		rememberWhen(s)

		nextTime := time.Now()
		nextTime = nextTime.AddDate(0, 0, 1)
		nextTime.Add(time.Duration(rand.Intn(int(24*time.Hour))) * time.Millisecond)
		os.WriteFile(nextTimeFileName, []byte(nextTime.Format(time.UnixDate)), 0777)
	}
}

func main() {
	token := strings.Replace(os.Getenv("DISCORD_AUTH"), "\"", "", -1)
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Printf("unable to create new discord instance")
		log.Fatal(err)
	}

	discord.Identify.Intents = discordgo.IntentsMessageContent
	// Register the messageCreate func as a callback for MessageCreate events.
	discord.AddHandler(commandSet.Handler)

	// Open a websocket connection to Discord and begin listening.
	err = discord.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	discord.UpdateGameStatus(1, "Reminding you of what matters")

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	go rememberWhenWorker(discord)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	discord.Close()

}
