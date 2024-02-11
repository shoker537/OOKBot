package data

import (
	config2 "OOKBot/config"
	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
	"log"
)

type MySQLData struct {
	Config    *config2.Config
	DB        *sqlx.DB
	PollCache *PollCache
}

func (d MySQLData) TeamOf(member discordgo.Member) *Team {
	for _, role := range member.Roles {
		team := Team{}
		err := d.DB.Get(&team, "SELECT * FROM teams WHERE role_id=? LIMIT 1", role)
		if err != nil {
			log.Println("Error receiving user team from database:" + err.Error())
			continue
		}
		if !team.Active {
			continue
		}
		return &team
	}
	return nil
}

func (d MySQLData) PollByMessage(messageId string) *Poll {
	for _, poll := range d.PollCache.Polls {
		if poll.MessageID == messageId {
			return poll
		}
	}
	poll := Poll{}
	err := d.DB.Get(&poll, "SELECT * FROM polls WHERE message_id=? LIMIT 1", messageId)
	if err != nil {
		log.Println("Error receiving poll by message from database:" + err.Error())
		return nil
	}
	poll.Options = d.GetPollAnswerOptions(poll.Id)
	d.PollCache.AddToCache(&poll)
	return &poll
}
func (d MySQLData) PollById(id uint16) *Poll {
	for _, poll := range d.PollCache.Polls {
		if poll.Id == id {
			return poll
		}
	}
	poll := Poll{}
	err := d.DB.Get(&poll, "SELECT * FROM polls WHERE id=? LIMIT 1", id)
	if err != nil {
		log.Println("Error receiving poll by message from database:" + err.Error())
		return nil
	}
	poll.Options = d.GetPollAnswerOptions(poll.Id)
	d.PollCache.AddToCache(&poll)
	return &poll
}

func (d MySQLData) GetPollAnswerOptions(pollId uint16) []PollOption {
	array := make([]PollOption, 0)
	err := d.DB.Select(&array, "SELECT * FROM polls_options WHERE poll_id=?", pollId)
	if err != nil {
		log.Println("Error receiving poll answers from database:" + err.Error())
		return nil
	}
	return array
}
