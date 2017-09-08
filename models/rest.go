package models

import "time"

type Rest_Guild struct {
	ID        string
	Name      string
	Icon      string
	OwnerID   string
	JoinedAt  time.Time
	BotPrefix string
	Features  Rest_Guild_Features
}

type Rest_Guild_Features struct {
	Levels_Badges  Rest_Feature_Levels_Badges
	RandomPictures Rest_Feature_RandomPictures
}

type Rest_User struct {
	ID            string
	Username      string
	AvatarHash    string
	Discriminator string
	Bot           bool
}

type Rest_Member struct {
	GuildID  string
	JoinedAt time.Time
	Nick     string
	Roles    []string
}

type Rest_Is_Member struct {
	IsMember bool
}

type Rest_Ranking struct {
	Ranks []Rest_Ranking_Rank_Item
	Count int
}

type Rest_Ranking_Rank_Item struct {
	User     Rest_User
	IsMember bool
	EXP      int64
	Level    int
	Ranking  int
}

type Rest_Feature_Levels_Badges struct {
	Count int
}

type Rest_Feature_RandomPictures struct {
	Count int
}

type Rest_RandomPictures_HistoryItem struct {
	Link      string
	SourceID  string
	PictureID string
	Filename  string
	GuildID   string
	Time      time.Time
}

const (
	Redis_Key_Feature_Levels_Badges  = "robyul2-discord:feature:levels-badges:server:%s"
	Redis_Key_Feature_RandomPictures = "robyul2-discord:feature:randompictures:server:%s"
)
