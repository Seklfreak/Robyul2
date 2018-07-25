package nugugame

import (
	"time"

	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
)

type nuguGame struct {
	UUID             string
	User             *discordgo.User
	ChannelID        string
	CorrectIdols     []*idols.Idol
	IncorrectIdols   []*idols.Idol
	WaitingForGuess  bool
	CurrentIdol      *idols.Idol
	Gender           string // girl, boy, mixed
	GameImageIndex   map[string]int
	RoundDelay       time.Duration
	GameType         string // idol, group
	IsMultigame      bool   // if true all messages in the channel will be account for
	GuessChannel     chan *discordgo.Message
	TimeoutChannel   *time.Timer
	LastRoundMessage *discordgo.Message
	Difficulty       string

	// Lives                int // amount of lives the user has left ?
}
