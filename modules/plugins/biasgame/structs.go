package biasgame

import (
	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
)

type singleBiasGame struct {
	User             *discordgo.User
	GuildID          string
	ChannelID        string
	RoundLosers      []*idols.Idol
	RoundWinners     []*idols.Idol
	BiasQueue        []*idols.Idol
	TopEight         []*idols.Idol
	GameWinnerBias   *idols.Idol
	IdolsRemaining   int
	LastRoundMessage *discordgo.Message
	ReadyForReaction bool   // used to make sure multiple reactions aren't counted
	Gender           string // girl, boy, mixed
	GameImageIndex   map[string]int
}

type multiBiasGame struct {
	CurrentRoundMessageId string // used to find game when reactions are added
	ChannelID             string
	RoundLosers           []*idols.Idol
	RoundWinners          []*idols.Idol
	BiasQueue             []*idols.Idol
	TopEight              []*idols.Idol
	GameWinnerBias        *idols.Idol
	IdolsRemaining        int
	LastRoundMessage      *discordgo.Message
	Gender                string // girl, boy, mixed
	UserIdsInvolved       []string
	RoundDelay            int
	GameIsRunning         bool
	guildID               string

	// a map of fileName => image array position. This is used to make sure that when a random image is selected for a game, that the same image is still used throughout the game
	GameImageIndex map[string]int
}

type rankingStruct struct {
	userId           string
	guildId          string
	amountOfGames    int
	idolWithMostWins string
	userName         string
}
