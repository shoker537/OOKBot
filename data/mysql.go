package data

import (
	config2 "OOKBot/config"
	"github.com/jmoiron/sqlx"
	"log"
)

type MySQLData struct {
	Config    *config2.Config
	DB        *sqlx.DB
	PollCache *PollCache
}

func (d MySQLData) TeamOf(userId string) *Team {
	team := Team{}
	err := d.DB.Get(&team, "SELECT * FROM teams WHERE id=(SELECT team_id FROM teams_members WHERE user_id=?) LIMIT 1", userId)
	if err != nil {
		log.Println("Error receiving user team from database:" + err.Error())
		return nil
	}
	if !team.Active {
		return nil
	}
	return &team
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
	d.PollCache.AddToCache(messageId, &poll)
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
