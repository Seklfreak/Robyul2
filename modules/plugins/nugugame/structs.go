package nugugame

import (
	"time"

	"github.com/globalsign/mgo/bson"

	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
)

type nuguGame struct {
	User                *discordgo.User
	ChannelID           string
	CorrectIdols        []*idols.Idol
	IncorrectIdols      []*idols.Idol
	WaitingForGuess     bool
	CurrentIdol         *idols.Idol
	Gender              string // girl, boy, mixed
	GameType            string // idol, group
	IsMultigame         bool   // if true all messages in the channel will be account for
	GuessChannel        chan *discordgo.Message
	TimeoutChannel      *time.Timer
	LastRoundMessage    *discordgo.Message
	Difficulty          string
	LivesRemaining      int
	UsersCorrectGuesses map[string][]bson.ObjectId // userid => []ids of idols they got right.  used in multi only
}

// nuguGameForCache only the information necessary to restore a game. this is
// whats saved to redis
type nuguGameForCache struct {
	UserId              string
	ChannelID           string
	CorrectIdols        []bson.ObjectId
	IncorrectIdols      []bson.ObjectId
	CurrentIdolId       bson.ObjectId
	Gender              string // girl, boy, mixed
	GameType            string // idol, group
	IsMultigame         bool   // if true all messages in the channel will be account for
	Difficulty          string
	LivesRemaining      int
	UsersCorrectGuesses map[string][]bson.ObjectId // userid => []ids of idols they got right.  used in multi only
}
