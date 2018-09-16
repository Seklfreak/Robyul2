package nugugame

import (
	"time"

	"github.com/globalsign/mgo/bson"

	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
)

type nuguGame struct {
	User                *discordgo.User
	CorrectIdols        []*idols.Idol
	IncorrectIdols      []*idols.Idol
	CurrentIdol         *idols.Idol
	WaitingForGuess     bool
	IsMultigame         bool // if true all messages in the channel will be account for
	ChannelID           string
	Gender              string // girl, boy, mixed
	GameType            string // idol, group
	Difficulty          string
	GuessChannel        chan *discordgo.Message
	LastRoundMessage    *discordgo.Message
	GuessTimeoutTimer   *time.Timer
	LivesRemaining      int
	UsersCorrectGuesses map[string][]bson.ObjectId // userid => []ids of idols they got right.  used in multi only
}

// nuguGameForCache is only the information necessary to restore a game.
// This is whats saved to redis
type nuguGameForCache struct {
	UserId              string
	ChannelID           string
	Gender              string // girl, boy, mixed
	GameType            string // idol, group
	Difficulty          string
	CorrectIdols        []bson.ObjectId
	IncorrectIdols      []bson.ObjectId
	CurrentIdolId       bson.ObjectId
	IsMultigame         bool // if true all messages in the channel will be account for
	LivesRemaining      int
	UsersCorrectGuesses map[string][]bson.ObjectId // userid => []ids of idols they got right.  used in multi only
}
