package data

import (
	config2 "OOKBot/config"
	"encoding/json"
	"github.com/bwmarrin/discordgo"
	"log"
	"reflect"
	"strconv"
	"time"
)

type PollCache struct {
	Polls         []*Poll
	OpenPolls     []*OpenUserPoll
	CreatingPolls []*CreatingPoll
}

func (cache *PollCache) CreatingPollOf(userId string) *CreatingPoll {
	for _, poll := range cache.CreatingPolls {
		if poll.UserId == userId {
			return poll
		}
	}
	return nil
}

func (cache *PollCache) AddToCache(poll *Poll) {
	if poll == nil {
		log.Println("Error adding Poll to cache: Poll is nil")
		return
	}
	for _, p := range cache.Polls {
		if p.Id == poll.Id {
			log.Println("Unable to cache poll " + strconv.Itoa(int(poll.Id)) + ": already cached")
			return
		}
	}
	cache.Polls = append(cache.Polls, poll)
}

func (cache *PollCache) PollByMessageId(messageId string) *Poll {
	for _, poll := range cache.Polls {
		if poll.MessageID == messageId {
			return poll
		}
	}
	return nil
}
func (cache *PollCache) PollById(id uint16) *Poll {
	for _, poll := range cache.Polls {
		if poll.Id == id {
			return poll
		}
	}
	return nil
}

func (cache *PollCache) OpenPollByChannelId(channelId string) *OpenUserPoll {
	for _, poll := range cache.OpenPolls {
		if poll.ChannelId == channelId {
			return poll
		}
	}
	return nil
}

func (cache *PollCache) AddOpenPoll(poll *OpenUserPoll) {
	cache.OpenPolls = append(cache.OpenPolls, poll)
}

func (cache *PollCache) RemoveOpenPoll(poll *OpenUserPoll) {
	index := -1
	for i, openPoll := range cache.OpenPolls {
		if reflect.DeepEqual(openPoll, poll) {
			index = i
			break
		}
	}
	if index == -1 {
		log.Println("Unable to delete open poll: no index found")
		return
	}
	cache.OpenPolls = append(cache.OpenPolls[:index], cache.OpenPolls[index+1:]...)
}
func (cache *PollCache) RemoveCreatingPollOfUser(user string) {
	newPolls := make([]*CreatingPoll, 0)
	for _, p := range cache.CreatingPolls {
		if p.UserId != user {
			newPolls = append(newPolls, p)
		}
	}
	cache.CreatingPolls = newPolls
}

type Poll struct {
	Id        uint16
	Title     string
	Json      *string
	StartsAt  time.Time `db:"starts_at"`
	EndsAt    time.Time `db:"ends_at"`
	MessageID string    `db:"message_id"`
	Options   []PollOption
}

func (poll *Poll) CreateEmbeds() []*discordgo.MessageEmbed {
	embeds := make([]*discordgo.MessageEmbed, 0)
	endsSeconds := poll.EndsAt.Add(-3 * time.Hour).Unix()
	embed := NewEmbed().SetTitle(poll.Title).SetDescription("**Завершение**: <t:" + strconv.FormatInt(endsSeconds, 10) + ":R>\n \n**Варианты ответа**:")

	for index, option := range poll.Options {
		embed.AddField(strconv.Itoa(index+1)+".", option.Name)
	}

	embeds = append(embeds, embed.MessageEmbed)

	if poll.Json != nil && len([]rune(*poll.Json)) > 0 {
		pollMessageEmbed := new(discordgo.MessageEmbed)
		err := json.Unmarshal([]byte(*poll.Json), pollMessageEmbed)
		if err != nil {
			log.Println(err)
			pollMessageEmbed = nil
		} else {
			embeds = append(embeds, pollMessageEmbed)
		}
	}

	return embeds
}

func (poll *Poll) CompliesRules(config *config2.Config, openPoll OpenUserPoll) bool {
	maxSameVotes := TeamMaxSameVotesCount(config, *openPoll.Poll, openPoll.Team)
	for _, option := range openPoll.Poll.Options {
		count := 0
		for _, selectedOptionId := range openPoll.Choices {
			if selectedOptionId == 0 {
				break
			}
			if uint16(selectedOptionId) == option.Id {
				count++
			}
		}
		if count > maxSameVotes {
			return false
		}
	}
	return true
}

type CreatingPoll struct {
	Title   *string
	Json    *string
	EndsAt  *time.Time
	Options []string
	UserId  string
}

func (poll *CreatingPoll) CreateEmbed() *discordgo.MessageEmbed {
	title := "Нет заголовка"
	if !(poll.Title == nil || len([]rune(*poll.Title)) == 0) {
		title = *poll.Title
	}
	builder := NewEmbed().SetTitle(title)

	description := "**Завершение**: "
	if poll.EndsAt == nil {
		description += "не указано"
	} else {
		description += poll.EndsAt.Format("02.01.2006 15:04")
	}
	description += "\n "

	if len(poll.Options) == 0 {
		description += "\nВарианты ответа не назначены"
	} else {
		for i, option := range poll.Options {
			builder.AddField("Вариант ответа #"+strconv.Itoa(i+1), option)
		}
	}
	builder.SetDescription(description)
	return builder.MessageEmbed
}

func (poll *CreatingPoll) CreateFullResponse() discordgo.InteractionResponseData {
	embeds := make([]*discordgo.MessageEmbed, 0)
	embeds = append(embeds, poll.CreateEmbed())
	if poll.Json != nil && len([]rune(*poll.Json)) != 0 {
		pollMessageEmbed := new(discordgo.MessageEmbed)
		err := json.Unmarshal([]byte(*poll.Json), pollMessageEmbed)
		if err != nil {
			log.Println(err)
			pollMessageEmbed = nil
		} else {
			embeds = append(embeds, pollMessageEmbed)
		}
	}

	return discordgo.InteractionResponseData{
		Content: "**Редактирование опроса**\nЗаполните все параметры, затем нажмите \"Сохранить\".",
		Flags:   discordgo.MessageFlagsEphemeral,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "✏️ Задать заголовок опроса",
						Style:    discordgo.SecondaryButton,
						CustomID: "manage_create-poll_set-title",
					},
					discordgo.Button{
						Label:    "➕ Добавить Embed",
						Style:    discordgo.SecondaryButton,
						CustomID: "manage_create-poll_add-embed",
					},
					discordgo.Button{
						Label:    "❌ Удалить Embed",
						Style:    discordgo.SecondaryButton,
						CustomID: "manage_create-poll_delete-embed",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "➕ Добавить вариант ответа",
						Style:    discordgo.SecondaryButton,
						CustomID: "manage_create-poll_add-option",
					},
					discordgo.Button{
						Label:    "❌ Удалить вариант ответа",
						Style:    discordgo.SecondaryButton,
						CustomID: "manage_create-poll_remove-option",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "⏰ Время окончания",
						Style:    discordgo.SecondaryButton,
						CustomID: "manage_create-poll_time-end",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "✅ Сохранить",
						Style:    discordgo.SuccessButton,
						CustomID: "manage_create-poll_save",
					},
					discordgo.Button{
						Label:    "❌ Отменить",
						Style:    discordgo.DangerButton,
						CustomID: "manage_create-poll_cancel",
					},
				},
			},
		},
		Embeds: embeds,
	}
}

type PollOption struct {
	Id     uint16
	PollId uint16 `db:"poll_id"`
	Name   string
}

type OpenUserPoll struct {
	UserId    string
	ChannelId string
	Poll      *Poll
	Team      Team
	StartedAt time.Time
	Choices   map[int]int
}
