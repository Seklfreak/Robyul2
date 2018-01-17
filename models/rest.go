package models

import (
	"time"
)

type SettingLevel string

var (
	ISO8601 = "2006-01-02T15:04:05-0700"
)

type Rest_Member_Guild struct {
	ID        string
	Name      string
	Icon      string
	OwnerID   string
	JoinedAt  time.Time
	BotPrefix string
	Features  Rest_Guild_Features
	Channels  []Rest_Channel
	Settings  Rest_Settings
	Status    Rest_Status_Member
}

type Rest_Guild struct {
	ID        string
	Name      string
	Icon      string
	OwnerID   string
	JoinedAt  time.Time
	BotPrefix string
	Features  Rest_Guild_Features
	Settings  Rest_Settings
	Channels  []Rest_Channel
}

type Rest_Settings struct {
	Strings []Rest_Setting_String
}

type Rest_Setting_String struct {
	Key    string
	Level  SettingLevel
	Values []string
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
	Modules        []Rest_Feature_Module
	Eventlog       Rest_Feature_Eventlog
}

type Rest_Feature_Module struct {
	Name string
	ID   ModulePermissionsModule
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

type Rest_Role struct {
	ID          string
	GuildID     string
	Name        string
	Managed     bool
	Mentionable bool
	Hoist       bool
	Color       string
	Position    int
	Permissions int
}

type Rest_Emoji struct {
	ID            string
	GuildID       string
	Name          string
	Managed       bool
	RequireColons bool
	Animated      bool
	APIName       string
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

type Rest_Feature_Eventlog struct {
	Enabled bool
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

type Rest_Statistics_Histogram_Three struct {
	Time   string // ISO 8601
	Count1 int64
	Count2 int64
	Count3 int64
}

type Rest_Statistics_Histogram_TwoSub struct {
	Time     string // ISO 8601
	Count1   int64
	Count2   int64
	SubItems []Rest_Statistics_Histogram_TwoSub_SubItem
}

type Rest_Statistics_Histogram_TwoSub_SubItem struct {
	Key   string
	Value int64
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
	Content        []string
	Attachments    []string
	AuthorID       string
	AuthorUsername string
	Embeds         int
	Deleted        bool
}

type Rest_VanityInvite_Invite struct {
	Code    string
	GuildID string
}

type Rest_Eventlog struct {
	Channels []Rest_Channel
	Users    []Rest_User
	Roles    []Rest_Role
	Entries  []Rest_Eventlog_Entry
	Emoji    []Rest_Emoji
	Guilds   []Rest_Guild
}

type Rest_Eventlog_Entry struct {
	CreatedAt      time.Time
	TargetID       string
	TargetType     string
	UserID         string
	ActionType     string
	Reason         string
	Changes        []ElasticEventlogChange
	Options        []ElasticEventlogOption
	WaitingForData bool
}

const (
	Redis_Key_Feature_Levels_Badges  = "robyul2-discord:feature:levels-badges:server:%s"
	Redis_Key_Feature_RandomPictures = "robyul2-discord:feature:randompictures:server:%s"
)
