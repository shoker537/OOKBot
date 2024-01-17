package handler

import (
	config2 "OOKBot/config"
	data2 "OOKBot/data"
	"encoding/json"
	"github.com/bwmarrin/discordgo"
	"log"
	"strconv"
	"strings"
	"time"
)

type InteractionHandler struct {
	Discord   *discordgo.Session
	Config    *config2.Config
	Guild     *discordgo.Guild
	MySQL     *data2.MySQLData
	PollCache *data2.PollCache
}

func (h *InteractionHandler) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionMessageComponent:
		h.buttonMessageClicked(s, i)
		break
	}
}

func (h *InteractionHandler) buttonMessageClicked(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customId := i.MessageComponentData().CustomID
	if strings.HasPrefix(customId, "option-") {
		h.onSelectOptionInVoting(customId, s, i)
		return
	}
	switch customId {
	case "vote_button":
		h.onClickVote(s, i)
		break
	}
}

func (h *InteractionHandler) onSelectOptionInVoting(customId string, s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionId, err := strconv.Atoi(strings.Split(customId, "-")[1])
	if err != nil {
		log.Println("Unable to parse option id from customid: " + err.Error())
		return
	}
	openPoll := h.PollCache.OpenPollByChannelId(i.Message.ChannelID)
	if openPoll == nil {
		log.Println("OpenPoll not found by channel " + i.Message.ChannelID)
		return
	}
	value, err := strconv.Atoi(i.MessageComponentData().Values[0])
	openPoll.Choices[uint8(optionId)] = uint8(value)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: nil,
	})
	if err != nil {
		log.Println("Error responding to interaction: " + err.Error())
	}
}

func (h *InteractionHandler) onClickVote(s *discordgo.Session, i *discordgo.InteractionCreate) {
	team := h.MySQL.TeamOf(i.Member.User.ID)
	if team == nil {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Команда отсутствует или отключена.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Println(err.Error())
		}
		return
	}
	clickedPoll := h.MySQL.PollByMessage(i.Message.ID)
	if clickedPoll == nil {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Не найден опрос, соответствующий данному сообщению.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Println(err.Error())
		}
		return
	}
	for _, openPoll := range h.PollCache.OpenPolls {
		if openPoll.UserId == i.Member.User.ID && openPoll.PollId == clickedPoll.Id {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "У вас уже открыт канал с заполнением бюллетеня.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				log.Println(err.Error())
			}
			return
		}
	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: nil,
	})
	if err != nil {
		log.Println(err.Error())
	}
	go h.createNewRoom(*team, *i.Member.User, clickedPoll)
}

func (h *InteractionHandler) createNewRoom(team data2.Team, user discordgo.User, poll *data2.Poll) {
	c, err := h.Discord.State.Channel(h.Config.VotingCategoryId)
	if c == nil || err != nil {
		c, err = h.Discord.Channel(h.Config.VotingCategoryId)
	}
	if err != nil {
		log.Println("Error getting category channel: " + err.Error())
		return
	}
	data := discordgo.GuildChannelCreateData{
		Name:     "бюллетень-" + (strings.Split(user.Username, "#")[0]),
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: c.ID,
	}
	channel, err := h.Discord.GuildChannelCreateComplex(h.Guild.ID, data)
	if err != nil {
		panic("Error creating channel: " + err.Error())
	}

	h.PollCache.AddOpenPoll(&data2.OpenUserPoll{
		UserId:    user.ID,
		ChannelId: channel.ID,
		PollId:    poll.Id,
		StartedAt: time.Now(),
		Choices:   make(map[uint8]uint8),
	})

	// CHANNEL CRETED MESSAGE

	embed := &discordgo.MessageEmbed{
		Title: "Добро пожаловать в вашу кабинку для голосования!",
		Description: "Давай заполним бюллетень.\n \nКабинка автоматически закроется <t:" + strconv.FormatInt(time.Now().Unix()+(1800), 10) + ":R>.\n" +
			"Ответы будут сохранены только после нажатия кнопки \"Отправить\".",
		Color: 1862612,
	}

	messageSend := discordgo.MessageSend{
		Content: "<@" + user.ID + ">",
		Embed:   embed,
	}

	_, err = h.Discord.ChannelMessageSendComplex(channel.ID, &messageSend)
	if err != nil {
		panic("Error sending message: " + err.Error())
	}
	votes := data2.TeamVotesCount(h.Config, team)
	sameVoices := 2
	options := h.MySQL.GetPollAnswerOptions(poll.Id)
	if len(options) < 4 && votes == 3 {
		votes--
		sameVoices--
	}

	// VOTINIG RULES REMINDER MESSAGE
	_, err = data2.NewEmbed().SetTitle("Напоминание о системе голосов").SetDescription("\nБольшие команды (с тремя голосами) могут выбирать один вариант только 2 раза (третий голос либо не используется, либо за другой вариант). В голосованиях с 3 или менее вариантами ответа у больших команд 2 голоса, оба за разные варианты (либо второй не используется).").AddField("Вы голосуете от лица команды", team.Name).AddField("Количество голосов:", strconv.Itoa(votes)).Send(h.Discord, channel.ID)
	if err != nil {
		log.Println("Error sending message: " + err.Error())
		return
	}

	_, err = data2.NewEmbed().SetDescription("Приступим к голосованию!").Send(h.Discord, channel.ID)
	if err != nil {
		log.Println("Error sending message: " + err.Error())
		return
	}
	// THE QUESTION MESSAGE
	pollMessageEmbed := new(discordgo.MessageEmbed)
	err = json.Unmarshal([]byte(poll.Json), pollMessageEmbed)
	if err != nil {
		log.Println(err)
		return
	}

	pollMessageSend := discordgo.MessageSend{
		Content: "Вопрос на повестке дня:",
		Embed:   pollMessageEmbed,
	}

	h.Discord.ChannelMessageSendComplex(channel.ID, &pollMessageSend)

	// VARIANTS WITH SELECT BOX

	pollVariantsMessageSend := discordgo.MessageSend{
		Content:    "Выберите варианты ответа, на которые хотите потратить свои голоса:",
		Components: []discordgo.MessageComponent{},
	}

	for i := 0; i < votes; i++ {
		selectMenuOptions := make([]discordgo.SelectMenuOption, 0)

		for _, option := range options {
			selectMenuOptions = append(selectMenuOptions, discordgo.SelectMenuOption{
				Label: option.Name,
				Value: strconv.Itoa(int(option.Id)),
			})
		}

		selectMenuOptions = append(selectMenuOptions, discordgo.SelectMenuOption{
			Label: "[Не использовать голос]",
			Value: "0",
		})

		pollVariantsMessageSend.Components = append(pollVariantsMessageSend.Components, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					MenuType:    discordgo.StringSelectMenu,
					CustomID:    "option-" + strconv.Itoa(i+1),
					Placeholder: "Использовать голос " + strconv.Itoa(i+1),
					Options:     selectMenuOptions,
				},
			},
		})
	}
	pollVariantsMessageSend.Components = append(pollVariantsMessageSend.Components, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Отправить",
				Style:    discordgo.SuccessButton,
				CustomID: "voting_save",
			},
			discordgo.Button{
				Label:    "Отменить",
				Style:    discordgo.DangerButton,
				CustomID: "voting_cancel",
			},
		},
	})

	_, err = h.Discord.ChannelMessageSendComplex(channel.ID, &pollVariantsMessageSend)
	if err != nil {
		log.Println("Error sending variants: " + err.Error())
		return
	}

}
