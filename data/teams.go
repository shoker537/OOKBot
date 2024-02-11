package data

import (
	config2 "OOKBot/config"
	"strconv"
)

type Team struct {
	Id     uint32
	Name   string
	Size   uint8
	Active bool
	RoleId string `db:"role_id"`
}

func TeamVotesCount(config *config2.Config, poll Poll, team Team) int {
	size := team.Size
	votes := 0
	fewOptions := len(poll.Options) < 4
	for {
		v, ok := config.Votes[strconv.Itoa(int(size))]
		if ok {
			votes = int(v)
			break
		}
		size--
	}
	if fewOptions && votes > 2 {
		return votes - 1
	}
	return votes
}

func TeamMaxSameVotesCount(config *config2.Config, poll Poll, team Team) int {
	size := team.Size
	votes := 0
	fewOptions := len(poll.Options) < 4
	for {
		v, ok := config.Votes[strconv.Itoa(int(size))]
		if ok {
			votes = int(v)
			break
		}
		size--
	}
	if votes == 1 {
		return votes
	}
	if fewOptions {
		return 1
	}
	if votes == 2 {
		return votes
	}
	return votes - 1
}
