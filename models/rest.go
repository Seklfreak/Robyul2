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
	Channels  []Rest_Channel
}

type Rest_Channel struct {
	ID       string
	GuildID  string
	Name     string
	ParentID string
	Type     string
	Topic    string
	Position int
}

type Website_Session_Data struct {
	DiscordUserID string
}

type Rest_Guild_Features struct {
	Levels_Badges  Rest_Feature_Levels_Badges
	RandomPictures Rest_Feature_RandomPictures
	Chatlog        Rest_Feature_Chatlog
	VanityInvite   Rest_Feature_VanityInvite
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

type Rest_Status_Member struct {
	IsMember                        bool
	IsBotAdmin                      bool
	IsNukeMod                       bool
	IsRobyulStaff                   bool
	IsBlacklisted                   bool
	IsGuildAdmin                    bool
	IsGuildMod                      bool
	HasGuildPermissionAdministrator bool
}

type Rest_Member_Guild struct {
	ID        string
	Name      string
	Icon      string
	OwnerID   string
	JoinedAt  time.Time
	BotPrefix string
	Features  Rest_Guild_Features
	Channels  []Rest_Channel
	Status    Rest_Status_Member
}

type Rest_Ranking struct {
	Ranks []Rest_Ranking_Rank_Item
	Count int
}

type Rest_Ranking_Rank_Item struct {
	User     Rest_User
	GuildID  string
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

type Rest_Feature_Chatlog struct {
	Enabled bool
}

type Rest_Feature_VanityInvite struct {
	VanityInviteName string
}

type Rest_RandomPictures_HistoryItem struct {
	Link      string
	SourceID  string
	PictureID string
	Filename  string
	GuildID   string
	Time      time.Time
}

type Rest_Statistics_Histogram struct {
	Time  string // ISO 8601
	Count int64
}

type Rest_Statistics_Histogram_Two struct {
	Time   string // ISO 8601
	Count1 int64
	Count2 int64
}

type Rest_Statistics_Count struct {
	Count int64
}

type Rest_Statitics_Bot struct {
	Users  int
	Guilds int
}

type Rest_Chatlog_Message struct {
	CreatedAt      time.Time
	ID             string
	Content        string
	Attachments    []string
	AuthorID       string
	AuthorUsername string
	Embeds         int
}

type Rest_VanityInvite_Invite struct {
	Code    string
	GuildID string
}

const (
	Redis_Key_Feature_Levels_Badges  = "robyul2-discord:feature:levels-badges:server:%s"
	Redis_Key_Feature_RandomPictures = "robyul2-discord:feature:randompictures:server:%s"
)
