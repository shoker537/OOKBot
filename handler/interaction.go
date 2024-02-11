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
		h.interactMessageComponent(s, i)
		break
	case discordgo.InteractionModalSubmit:
		h.modalSubmit(s, i)
		break
	}
}

func (h *InteractionHandler) modalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customId := i.ModalSubmitData().CustomID
	switch customId {
	case "manage_create-poll_set-title_modal":
		h.createPollSetTitleModalComplete(s, i)
		break
	case "manage_create-poll_set-embed_modal":
		h.createPollSetEmbedModalComplete(s, i)
		break
	case "manage_create-poll_add-option_modal":
		h.createPollAddOptionModalComplete(s, i)
		break
	case "manage_create-poll_time-end_modal":
		h.createPollSetTimeEndModalComplete(s, i)
		break
	}
}
func (h *InteractionHandler) interactMessageComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customId := i.MessageComponentData().CustomID
	if strings.HasPrefix(customId, "option-") {
		h.onSelectOptionInVoting(customId, s, i)
		return
	}
	log.Println("Interaction received: " + customId)
	switch customId {
	case "vote_button":
		h.onClickVoteCreateChannel(s, i)
		break
	case "voting_save":
		h.saveVote(s, i)
		break
	case "voting_cancel":
		openPoll := h.PollCache.OpenPollByChannelId(i.Message.ChannelID)
		if openPoll == nil {
			log.Println("OpenPoll not found by channel " + i.Message.ChannelID)
		} else {
			h.PollCache.RemoveOpenPoll(openPoll)
		}
		_, err := s.ChannelDelete(i.ChannelID)
		if err != nil {
			log.Println("Unable to delete channel: " + err.Error())
			break
		}
		break
	case "manage_create-team":
		h.createTeam(s, i)
		break
	case "manage_create-poll":
		h.createPoll(s, i)
		break
	case "manage_create-poll_set-title":
		h.createPollSetTitle(s, i)
		break
	case "manage_create-poll_add-embed":
		h.createPollSetEmbed(s, i)
		break
	case "manage_create-poll_add-option":
		h.createPollAddOption(s, i)
		break
	case "manage_create-poll_delete-embed":
		h.createPollDeleteEmbed(s, i)
		break
	case "manage_create-poll_remove-option":
		h.createPollDeleteOption(s, i)
		break
	case "manage_create-poll_remove-option_select-menu":
		h.createPollRemoveOptionSelected(s, i)
		break
	case "manage_create-poll_cancel":
		h.createPollCancel(s, i)
		break
	case "manage_create-poll_save":
		h.createPollSubmit(s, i)
		break
	case "manage_create-poll_remove-option_cancel":
		h.createPollRemoveOptionCancel(s, i)
		break
	case "manage_create-poll_time-end":
		h.createPollSetTimeEnd(s, i)
		break
	}
}

func (h *InteractionHandler) createPollRemoveOptionCancel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	rd := creatingPoll.CreateFullResponse()
	reactInteractionWithUpdate(s, i, rd)
}

func (h *InteractionHandler) createPollCancel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	h.PollCache.RemoveCreatingPollOfUser(i.Member.User.ID)
	rd := discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{
			data2.NewEmbed().SetTitle("❌ Создание отменено").SetDescription("Можете удалить это сообщение.").MessageEmbed,
		},
	}
	reactInteractionWithUpdate(s, i, rd)
}

func (h *InteractionHandler) createPollSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	if creatingPoll.Title == nil || len([]rune(*creatingPoll.Title)) == 0 {
		reactInteractionWithMessageString(s, i, "У опроса должен быть указан заголовок.")
		return
	}
	if creatingPoll.EndsAt == nil {
		reactInteractionWithMessageString(s, i, "У опроса должено быть указано время завершения.")
		return
	}
	if len(creatingPoll.Options) < 2 {
		reactInteractionWithMessageString(s, i, "У опроса должено быть минимум 2 варианта ответа.")
		return
	}
	rs, err := h.MySQL.DB.Exec("INSERT INTO polls SET title=?, json=?, starts_at=NOW(), ends_at=?, message_id=\"\"", creatingPoll.Title, creatingPoll.Json, creatingPoll.EndsAt)
	if err != nil {
		log.Println(err)
		return
	}
	id, err := rs.LastInsertId()
	if err != nil {
		log.Println(err)
		return
	}
	for _, option := range creatingPoll.Options {
		_, err = h.MySQL.DB.Exec("INSERT INTO polls_options SET poll_id=?, name=?", id, option)
		if err != nil {
			log.Println(err)
			return
		}
	}
	h.PollCache.RemoveCreatingPollOfUser(i.Member.User.ID)

	poll := h.MySQL.PollById(uint16(id))
	if poll == nil {
		log.Println("Not found poll by id " + strconv.Itoa(int(id)))
		return
	}

	messageSend := discordgo.MessageSend{
		Embeds: poll.CreateEmbeds(),
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Проголосовать",
					Style:    discordgo.PrimaryButton,
					CustomID: "vote_button",
				},
			}},
		},
	}
	msg, err := s.ChannelMessageSendComplex(h.Config.PollListChannelId, &messageSend)
	if err != nil {
		log.Println(err)
		return
	}
	poll.MessageID = msg.ID
	_, err = h.MySQL.DB.Exec("UPDATE polls SET message_id=? WHERE id=? LIMIT 1", msg.ID, poll.Id)
	if err != nil {
		log.Println(err)
		return
	}

	rd := discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{
			data2.NewEmbed().SetTitle("✅ Опрос создан!").SetDescription("Можете удалить это сообщение.").MessageEmbed,
		},
	}

	reactInteractionWithUpdate(s, i, rd)
}

func (h *InteractionHandler) createPollRemoveOptionSelected(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	selected := i.MessageComponentData().Values[0]
	index, err := strconv.Atoi(selected)
	if err != nil {
		log.Println(err)
		return
	}
	creatingPoll.Options = append(creatingPoll.Options[:index], creatingPoll.Options[index+1:]...)

	rd := creatingPoll.CreateFullResponse()
	reactInteractionWithUpdate(s, i, rd)
}

func (h *InteractionHandler) createPollSetTimeEndModalComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	input := i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput)
	value := input.Value
	endsAt, err := time.Parse("02.01.2006 15:04", value)
	if err != nil {
		log.Println(err)
		return
	}
	creatingPoll.EndsAt = &endsAt
	rd := creatingPoll.CreateFullResponse()
	reactInteractionWithUpdate(s, i, rd)
}

func (h *InteractionHandler) createPollAddOptionModalComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	input := i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput)
	value := input.Value
	creatingPoll.Options = append(creatingPoll.Options, value)

	rd := creatingPoll.CreateFullResponse()
	reactInteractionWithUpdate(s, i, rd)
}

func (h *InteractionHandler) createPollSetEmbedModalComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	input := i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput)
	value := input.Value
	creatingPoll.Json = &value

	rd := creatingPoll.CreateFullResponse()
	reactInteractionWithUpdate(s, i, rd)
}

func (h *InteractionHandler) createPollSetTitleModalComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	input := i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput)
	value := input.Value
	creatingPoll.Title = &value

	rd := creatingPoll.CreateFullResponse()
	reactInteractionWithUpdate(s, i, rd)
}

func (h *InteractionHandler) createPollDeleteOption(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	if len(creatingPoll.Options) == 0 {
		reactInteractionWithMessageString(s, i, "Список вариантов ответа пуст.")
		return
	}
	menuOptions := make([]discordgo.SelectMenuOption, 0)
	for index, option := range creatingPoll.Options {
		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label: option,
			Value: strconv.Itoa(index),
		})
	}

	rd := creatingPoll.CreateFullResponse()
	newComponents := make([]discordgo.MessageComponent, 0)
	row1 := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				MenuType:    discordgo.StringSelectMenu,
				CustomID:    "manage_create-poll_remove-option_select-menu",
				MaxValues:   1,
				Placeholder: "Выберите вариант для удаления",
				Options:     menuOptions,
			},
		},
	}
	row2 := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Style:    discordgo.DangerButton,
				CustomID: "manage_create-poll_remove-option_cancel",
				Label:    "❌ Отменить удаление",
			},
		},
	}
	newComponents = append(newComponents, row1)
	newComponents = append(newComponents, row2)
	rd.Components = newComponents
	reactInteractionWithMessage(s, i, rd)
}

func (h *InteractionHandler) createPollSetTimeEnd(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	value := ""
	if creatingPoll.EndsAt != nil {
		value = creatingPoll.EndsAt.Format("02.01.2006 15:04")
	}
	rd := discordgo.InteractionResponseData{
		Title:    "Время окончания опроса (МСК)",
		CustomID: "manage_create-poll_time-end_modal",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.TextInput{
					CustomID:  "manage_create-poll_time-end_modal_input",
					Label:     "Формат: 31.12.2024 08:50",
					Style:     discordgo.TextInputShort,
					Value:     value,
					Required:  true,
					MinLength: 16,
					MaxLength: 16,
				},
			}},
		},
	}
	reactInteractionWithModal(s, i, rd)
}
func (h *InteractionHandler) createPollAddOption(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	value := ""
	if creatingPoll.Json != nil {
		value = *creatingPoll.Json
	}
	rd := discordgo.InteractionResponseData{
		Title:    "Добавить вариант ответа",
		CustomID: "manage_create-poll_add-option_modal",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.TextInput{
					CustomID:  "manage_create-poll_add-option_modal_input",
					Label:     "Новый вариант ответа",
					Style:     discordgo.TextInputShort,
					Value:     value,
					Required:  true,
					MinLength: 1,
					MaxLength: 255,
				},
			}},
		},
	}
	reactInteractionWithModal(s, i, rd)
}

func (h *InteractionHandler) createPollSetEmbed(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	value := ""
	if creatingPoll.Json != nil {
		value = *creatingPoll.Json
	}
	rd := discordgo.InteractionResponseData{
		Title:    "Задать Embed опроса",
		Content:  "Это может быть описание или причины опроса, важные дополнения или картинка.",
		CustomID: "manage_create-poll_set-embed_modal",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.TextInput{
					CustomID:    "manage_create-poll_set-embed_modal_input",
					Label:       "Embed",
					Style:       discordgo.TextInputParagraph,
					Value:       value,
					Placeholder: "Сюда надо вставить JSON",
					Required:    true,
					MinLength:   2,
					MaxLength:   4000,
				},
			}},
		},
	}
	reactInteractionWithModal(s, i, rd)
}

func (h *InteractionHandler) createPollDeleteEmbed(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	creatingPoll.Json = nil

	rd := creatingPoll.CreateFullResponse()
	reactInteractionWithUpdate(s, i, rd)
}

func (h *InteractionHandler) createPollSetTitle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creatingPoll := h.PollCache.CreatingPollOf(i.Member.User.ID)
	if creatingPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден изменяемый опрос!")
		return
	}
	value := ""
	if creatingPoll.Title != nil {
		value = *creatingPoll.Title
	}
	rd := discordgo.InteractionResponseData{
		Title:    "Изменить заголовок опроса",
		CustomID: "manage_create-poll_set-title_modal",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.TextInput{
					CustomID:    "manage_create-poll_set-title_modal_input",
					Label:       "Заголовок опроса",
					Style:       discordgo.TextInputShort,
					Value:       value,
					Placeholder: "Новый заголовок",
					Required:    true,
					MinLength:   1,
					MaxLength:   255,
				},
			}},
		},
	}
	reactInteractionWithModal(s, i, rd)
}
func (h *InteractionHandler) createPoll(s *discordgo.Session, i *discordgo.InteractionCreate) {
	h.PollCache.RemoveCreatingPollOfUser(i.Member.User.ID)
	poll := &data2.CreatingPoll{
		UserId: i.Member.User.ID,
	}
	h.PollCache.CreatingPolls = append(h.PollCache.CreatingPolls, poll)
	rd := poll.CreateFullResponse()
	reactInteractionWithMessage(s, i, rd)
}
func (h *InteractionHandler) createTeam(s *discordgo.Session, i *discordgo.InteractionCreate) {
	sizeOptions := []discordgo.SelectMenuOption{
		{
			Label:   "Маленькая",
			Value:   "small",
			Default: true,
		},
		{
			Label:   "Средняя",
			Value:   "medium",
			Default: false,
		},
		{
			Label:   "Большая",
			Value:   "big",
			Default: false,
		},
	}

	minValues := 1
	rd := discordgo.InteractionResponseData{
		Content: "**Записать новую команду**\nЗаполните поля ниже, затем нажмите \"Создать\".",
		Flags:   discordgo.MessageFlagsEphemeral,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						MenuType:    discordgo.RoleSelectMenu,
						CustomID:    "manage_create-team_select-role",
						Placeholder: "Выберите роль новой команды",
						MinValues:   &minValues,
						MaxValues:   1,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "manage_create-team_select-size",
						Placeholder: "Выберите количество игроков",
						Options:     sizeOptions,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Создать",
						Style:    discordgo.SuccessButton,
						CustomID: "manage_create-team_save",
					},
					discordgo.Button{
						Label:    "Отменить",
						Style:    discordgo.DangerButton,
						CustomID: "manage_create-team_cancel",
					},
				},
			},
		},
	}
	reactInteractionWithMessage(s, i, rd)
}

func (h *InteractionHandler) saveVote(s *discordgo.Session, i *discordgo.InteractionCreate) {
	openPoll := h.PollCache.OpenPollByChannelId(i.Message.ChannelID)
	if openPoll == nil {
		log.Println("OpenPoll not found by channel " + i.Message.ChannelID)
		return
	}

	if !openPoll.Poll.CompliesRules(h.Config, *openPoll) {
		reactInteractionWithMessageString(s, i, "Вы превысили ограничение одинаковых ответов. Это противоречит правилам голосования.")
		return
	}

	_, err := h.MySQL.DB.Exec("DELETE FROM polls_votes WHERE team_id=? AND poll_id=?", openPoll.Team.Id, openPoll.Poll.Id)
	if err != nil {
		log.Println("Error removing team votes: " + err.Error())
		reactInteractionWithMessageString(s, i, "Ошибка при обработке голосов. #1")
		return
	}
	for _, value := range openPoll.Choices {
		if value > 0 {
			_, err := h.MySQL.DB.Exec("INSERT INTO polls_votes SET team_id=?, poll_id=?, vote=?", openPoll.Team.Id, openPoll.Poll.Id, value)
			if err != nil {
				log.Println("Error adding team votes: " + err.Error())
				reactInteractionWithMessageString(s, i, "Ошибка при обработке голосов. #2")
				return
			}
		}
	}
	reactInteractionWithSuccess(s, i) //todo: replace with message edit to disable all components
	data2.NewEmbed().SetTitle("Так и запишем!").SetDescription("Спасибо, что приняли участие в голосовнии.\n \nКанал закроется <t:"+strconv.FormatInt(time.Now().Unix()+(6), 10)+":R>").SetColor(1862612).Send(s, openPoll.ChannelId)
	h.PollCache.RemoveOpenPoll(openPoll)
	go h.delChannelAfterVoting(openPoll.ChannelId)
}

func (h *InteractionHandler) delChannelAfterVoting(channelId string) {
	time.Sleep(5 * time.Second)
	_, err := h.Discord.ChannelDelete(channelId)
	if err != nil {
		log.Println("Error deleting channel: " + err.Error())
		return
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

	optionKey := optionId
	optionValue := value
	openPoll.Choices[optionKey] = optionValue
	if !openPoll.Poll.CompliesRules(h.Config, *openPoll) {
		reactInteractionWithMessageString(s, i, "Вы превысили ограничение одинаковых ответов. Это противоречит правилам голосования. Такой ответ не будет принят.")
		return
	}

	reactInteractionWithSuccess(s, i)
}

// TODO: switch to Modal when Discord releases SelectMenus there
func (h *InteractionHandler) onClickVoteShowModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	team := h.MySQL.TeamOf(*i.Member)
	if team == nil {
		reactInteractionWithMessageString(s, i, "Клан отсутствует или исключён из голосования.")
		log.Println("Team is null for user " + i.Member.User.ID)
		return
	}
	clickedPoll := h.MySQL.PollByMessage(i.Message.ID)
	if clickedPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден опрос, соответствующий данному сообщению.")
		return
	}
	votes := data2.TeamVotesCount(h.Config, *clickedPoll, *team)
	rows := make([]discordgo.MessageComponent, 0)
	for i := 0; i < votes; i++ {
		selectMenuOptions := make([]discordgo.SelectMenuOption, 0)

		for _, option := range clickedPoll.Options {
			selectMenuOptions = append(selectMenuOptions, discordgo.SelectMenuOption{
				Label: option.Name,
				Value: strconv.Itoa(int(option.Id)),
			})
		}

		selectMenuOptions = append(selectMenuOptions, discordgo.SelectMenuOption{
			Label: "[Не использовать голос]",
			Value: "0",
		})

		rows = append(rows, discordgo.ActionsRow{
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
	data := discordgo.InteractionResponseData{
		CustomID:   "poll_modal_" + i.Member.User.ID,
		Title:      "Участие в голосовании",
		Components: rows,
	}
	reactInteractionWithModal(s, i, data)
}

func (h *InteractionHandler) onClickVoteCreateChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	team := h.MySQL.TeamOf(*i.Member)
	if team == nil {
		reactInteractionWithMessageString(s, i, "Клан отсутствует или исключён из голосования.")
		log.Println("Team is null for user " + i.Member.User.ID)
		return
	}
	clickedPoll := h.MySQL.PollByMessage(i.Message.ID)
	if clickedPoll == nil {
		reactInteractionWithMessageString(s, i, "Не найден опрос, соответствующий данному сообщению.")
		return
	}
	for _, openPoll := range h.PollCache.OpenPolls {
		if openPoll.Poll.Id == clickedPoll.Id {
			if openPoll.UserId == i.Member.User.ID {
				reactInteractionWithMessageString(s, i, "У вас уже открыт канал с заполнением бюллетеня.")
				return
			}
			if team.Id == openPoll.Team.Id {
				reactInteractionWithMessageString(s, i, "Кто-то из вашей команды уже заполняет этот бюллетень.")
				return
			}
		}
	}
	reactInteractionWithSuccess(s, i)
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
		Name:     "кабинка-" + (strings.Split(user.Username, "#")[0]),
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: c.ID,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{
				ID:   h.Config.EveryoneRole,
				Type: discordgo.PermissionOverwriteTypeRole,
				Deny: 1024,
			},
			{
				ID:    user.ID,
				Type:  discordgo.PermissionOverwriteTypeMember,
				Allow: 1024,
			},
			{
				ID:    h.Discord.State.User.ID,
				Type:  discordgo.PermissionOverwriteTypeMember,
				Allow: 1024,
			},
		},
	}
	channel, err := h.Discord.GuildChannelCreateComplex(h.Guild.ID, data)
	if err != nil {
		panic("Error creating channel: " + err.Error())
	}

	h.PollCache.AddOpenPoll(&data2.OpenUserPoll{
		UserId:    user.ID,
		ChannelId: channel.ID,
		Poll:      poll,
		StartedAt: time.Now(),
		Choices:   make(map[int]int),
		Team:      team,
	})

	// CHANNEL CREATED MESSAGE

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
		log.Println("Error sending message: " + err.Error())
		return
	}
	votes := data2.TeamVotesCount(h.Config, *poll, team)
	options := poll.Options

	// VOTINIG RULES REMINDER MESSAGE
	_, err = data2.NewEmbed().SetTitle("Напоминание о системе голосов").AddField("Вы голосуете от лица команды", team.Name).AddField("Количество голосов:", strconv.Itoa(votes)).AddField("Максимум одинаковых голосов:", strconv.Itoa(data2.TeamMaxSameVotesCount(h.Config, *poll, team))).Send(h.Discord, channel.ID)
	if err != nil {
		log.Println("Error sending message: " + err.Error())
		return
	}

	embeds := make([]*discordgo.MessageEmbed, 0)

	embeds = append(embeds, data2.NewEmbed().SetTitle(poll.Title).MessageEmbed)

	// CUSTOM QUESTION MESSAGE
	if poll.Json != nil && len([]rune(*poll.Json)) > 0 {
		pollMessageEmbed := new(discordgo.MessageEmbed)
		err = json.Unmarshal([]byte(*poll.Json), pollMessageEmbed)
		if err != nil {
			log.Println(err)
		} else {
			embeds = append(embeds, pollMessageEmbed)
		}
	}

	pollMessageSend := discordgo.MessageSend{
		Content: "Вопрос на повестке дня:",
		Embeds:  embeds,
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

func reactInteractionWithSuccess(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: nil,
	})
	if err != nil {
		log.Println("Error responding to interaction: " + err.Error())
	}
}
func reactInteractionWithMessageString(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Println(err.Error())
	}
}
func reactInteractionWithModal(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.InteractionResponseData) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &data,
	})
	if err != nil {
		log.Println(err.Error())
	}
}
func reactInteractionWithUpdate(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.InteractionResponseData) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &data,
	})
	if err != nil {
		log.Println(err.Error())
	}
}
func reactInteractionWithMessage(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.InteractionResponseData) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &data,
	})
	if err != nil {
		log.Println(err.Error())
	}
}

type FillingNewTeam struct {
	RoleId string
	Size   string
	UserId string
}
