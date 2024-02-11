package main

import (
	conf "OOKBot/config"
	"OOKBot/data"
	"OOKBot/handler"
	"encoding/json"
	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var mysqlData *data.MySQLData
var ds *discordgo.Session
var guild *discordgo.Guild
var config *conf.Config
var pollCache *data.PollCache

func main() {
	pollCache = &data.PollCache{
		Polls:     make([]*data.Poll, 0),
		OpenPolls: make([]*data.OpenUserPoll, 0),
	}
	loadConfig()
	loadDatabase()
	connectDiscord()

	go runTimedTick()

	// wait for sigint
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	// on disable
	log.Println("Closing Discord connection...")
	ds.Close()
}

func loadConfig() {
	file, _ := os.Open("config.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	config = &conf.Config{}
	err := decoder.Decode(&config)
	if err != nil {
		panic("Unable to load config: " + err.Error())
	}
	log.Println("Loaded config!")
}

func loadDatabase() {
	db, err := sqlx.Open("mysql", config.Mysql.User+":"+config.Mysql.Password+"@tcp("+config.Mysql.Host+":"+strconv.Itoa(int(config.Mysql.Port))+")/"+config.Mysql.Database+"?parseTime=true")
	if err != nil {
		panic("Unable to connect to the database: " + err.Error())
	}
	mysqlData = &data.MySQLData{
		Config:    config,
		DB:        db,
		PollCache: pollCache,
	}
	log.Println("Loaded MySQL!")
}

func connectDiscord() {
	ds = NewDiscordSession()
	guild1, err := ds.Guild(config.GuildId)
	if err != nil {
		panic("Error finding guild: " + err.Error())
	}
	guild = guild1
	ds.Identify.Intents = discordgo.IntentsAllWithoutPrivileged
	interactionHandler := handler.InteractionHandler{
		Discord:   ds,
		Config:    config,
		Guild:     guild,
		MySQL:     mysqlData,
		PollCache: pollCache,
	}
	ds.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		interactionHandler.HandleInteraction(s, i)
	})
	ds.AddHandler(func(s *discordgo.Session, i *discordgo.Ready) {
		botReady(s)
	})
	err = ds.Open()
	if err != nil {
		panic("Error opening session: " + err.Error())
	}
	log.Println("Connection to Discord created!")
}

func NewDiscordSession() *discordgo.Session {
	ds, err := discordgo.New("Bot " + config.BotToken)
	if err != nil {
		log.Println("Unable to connect to Discord")
		return nil
	}
	ds.ShouldReconnectOnError = true
	return ds
}

func clearVotingCategory(s *discordgo.Session) {
	clearCategory(s, config.VotingCategoryId)
}

func clearMessagesInChannel(s *discordgo.Session, channelId string) {
	for {
		messages, err := s.ChannelMessages(channelId, 100, "", "", "")
		if err != nil {
			log.Println("Error reading messages of " + channelId + ": " + err.Error())
			return
		}
		if len(messages) == 0 {
			return
		}
		for _, message := range messages {
			err = s.ChannelMessageDelete(channelId, message.ID)
			if err != nil {
				log.Println("Unable to delete message " + message.ID + ": " + err.Error())
			}
		}
	}
}

func clearCategory(s *discordgo.Session, categoryId string) {
	channel, err := s.Channel(categoryId)
	if err != nil {
		log.Println("Error getting voting category: " + err.Error())
		return
	}
	children := categoryChannels(s, channel.ID)
	log.Println(len(children))
	for _, member := range children {
		_, err = s.ChannelDelete(member.ID)
		if err != nil {
			log.Println("Error removing channel: " + err.Error())
			return
		}
		log.Println("Removed channel " + member.ID)
	}
}

func categoryChannels(s *discordgo.Session, categoryId string) []discordgo.Channel {
	array := make([]discordgo.Channel, 0)
	category, err := s.Channel(config.VotingCategoryId)
	if err != nil {
		log.Println("Error getting category: " + err.Error())
		return array
	}
	g, err := s.Guild(category.GuildID)
	if err != nil {
		log.Println("Error getting guild of a category: " + err.Error())
		return array
	}
	channels, err := s.GuildChannels(g.ID)
	for _, channel := range channels {
		if channel != nil && channel.ParentID == categoryId {
			array = append(array, *channel)
		}
	}
	return array
}

func botReady(s *discordgo.Session) {
	log.Println("Bot is ready!")
	_, err := s.Channel(config.PollListChannelId)
	if err != nil {
		panic("Unable to get polls channel!")
	}
	clearVotingCategory(s)
	clearMessagesInChannel(s, config.ManagePollsChannel)
	setupManagePollsChannel(s)
}

func setupManagePollsChannel(s *discordgo.Session) {
	msgsend := discordgo.MessageSend{
		Embed: data.NewEmbed().SetTitle("Управление ботом ООК").SetColor(1862612).MessageEmbed,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Создать команду",
						Style:    discordgo.SuccessButton,
						CustomID: "manage_create-team",
					},
					discordgo.Button{
						Label:    "Изменить команду",
						Style:    discordgo.SecondaryButton,
						CustomID: "manage_edit-team",
					},
					discordgo.Button{
						Label:    "Удалить команду",
						Style:    discordgo.DangerButton,
						CustomID: "manage_delete-team",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Создать голосование",
						Style:    discordgo.SuccessButton,
						CustomID: "manage_create-poll",
					},
					discordgo.Button{
						Label:    "Изменить голосование",
						Style:    discordgo.SecondaryButton,
						CustomID: "manage_edit-poll",
					},
					discordgo.Button{
						Label:    "Удалить голосование",
						Style:    discordgo.DangerButton,
						CustomID: "manage_delete-poll",
					},
				},
			},
		},
	}

	s.ChannelMessageSendComplex(config.ManagePollsChannel, &msgsend)
}

func runTimedTick() {
	for {
		time.Sleep(5 * time.Second)
		timedTick()
	}
}

func timedTick() {
	pollsToDelete := make([]*data.OpenUserPoll, 0)
	for _, poll := range pollCache.OpenPolls {
		if time.Now().Unix()-poll.StartedAt.Unix() >= 1800 {
			pollsToDelete = append(pollsToDelete, poll)
		}
	}
	if len(pollsToDelete) == 0 {
		return
	}
	for _, poll := range pollsToDelete {
		pollCache.RemoveOpenPoll(poll)
		_, err := ds.ChannelDelete(poll.ChannelId)
		if err != nil {
			log.Println("Unable to automatically delete channel: " + err.Error())
		}
	}
}
