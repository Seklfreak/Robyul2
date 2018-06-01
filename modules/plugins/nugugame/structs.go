package nugugame

import (
	"time"

	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
)

type nuguGame struct {
	UUID              string
	User              *discordgo.User
	ChannelID         string
	CorrectIdols      []*idols.Idol
	IncorrectIdols    []*idols.Idol
	WaitingForMessage bool
	CurrentIdol       *idols.Idol
	Gender            string // girl, boy, mixed
	GameImageIndex    map[string]int
	RoundDelay        time.Duration
	GameType          string // idol, group
	IsMultigame       bool   // if true all messages in the channel will be account for
	LastRoundMessage  *discordgo.Message

	// Lives                int // amount of lives the user has left ?
}
