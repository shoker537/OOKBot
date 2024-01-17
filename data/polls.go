package data

import (
	"log"
	"strconv"
	"time"
)

type PollCache struct {
	Polls     []*Poll
	OpenPolls []*OpenUserPoll
}

func (cache *PollCache) AddToCache(id string, poll *Poll) {
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

type Poll struct {
	Id        uint16
	Json      string
	StartsAt  time.Time `db:"starts_at"`
	EndsAt    time.Time `db:"ends_at"`
	MessageID string    `db:"message_id"`
}

type PollOption struct {
	Id     uint16
	PollId uint16 `db:"poll_id"`
	Name   string
}

type OpenUserPoll struct {
	UserId    string
	ChannelId string
	PollId    uint16
	StartedAt time.Time
	Choices   map[uint8]uint8
}
