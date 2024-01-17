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
}

func TeamVotesCount(config *config2.Config, team Team) int {
	size := team.Size
	votes := 0
	for {
		v, ok := config.Votes[strconv.Itoa(int(size))]
		if ok {
			votes = int(v)
			break
		}
		size--
	}
	return votes
}
