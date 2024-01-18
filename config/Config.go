package config

type Config struct {
	Mysql             MySQLConfig
	BotToken          string `json:"bot-token"`
	VotingCategoryId  string `json:"voting-category-id"`
	PollListChannelId string `json:"poll-list-channel-id"`
	GuildId           string `json:"guild-id"`
	Votes             map[string]uint8
	EveryoneRole      string `json:"everyone-role"`
}

type MySQLConfig struct {
	Host     string
	Port     uint32
	User     string
	Password string
	Database string
}
