package biasgame

import (
	"github.com/bwmarrin/discordgo"
)

type biasImage struct {
	ImageBytes []byte
	HashString string
	ObjectName string
}

type biasChoice struct {
	BiasName     string
	GroupName    string
	Gender       string
	NameAndGroup string
	BiasImages   []biasImage
}

type singleBiasGame struct {
	User             *discordgo.User
	ChannelID        string
	RoundLosers      []*biasChoice
	RoundWinners     []*biasChoice
	BiasQueue        []*biasChoice
	TopEight         []*biasChoice
	GameWinnerBias   *biasChoice
	IdolsRemaining   int
	LastRoundMessage *discordgo.Message
	ReadyForReaction bool   // used to make sure multiple reactions aren't counted
	Gender           string // girl, boy, mixed

	// a map of idol name and group => image array position. This is used to make sure that when a random image is selected for a game, that the same image is still used throughout the game
	GameImageIndex map[string]int
}

type multiBiasGame struct {
	CurrentRoundMessageId string // used to find game when reactions are added
	ChannelID             string
	RoundLosers           []*biasChoice
	RoundWinners          []*biasChoice
	BiasQueue             []*biasChoice
	TopEight              []*biasChoice
	GameWinnerBias        *biasChoice
	IdolsRemaining        int
	LastRoundMessage      *discordgo.Message
	Gender                string // girl, boy, mixed
	UserIdsInvolved       []string
	RoundDelay            int
	GameIsRunning         bool

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
