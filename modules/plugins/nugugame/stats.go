package nugugame

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"
)

// recordNuguGame saves the nugugame to mongo
func recordNuguGame(game *nuguGame) {
	defer helpers.Recover() // this func is commonly called via coroutine

	// get guildID from game channel
	channel, _ := cache.GetSession().State.Channel(game.ChannelID)
	guild, err := cache.GetSession().State.Guild(channel.GuildID)
	if err != nil {
		fmt.Println("Error getting guild when recording stats")
		return
	}

	// get id of all idols
	var correctIdolIds []bson.ObjectId
	var incorrectIdolIds []bson.ObjectId
	for _, idol := range game.CorrectIdols {
		correctIdolIds = append(correctIdolIds, idol.ID)
	}
	for _, idol := range game.IncorrectIdols {
		incorrectIdolIds = append(incorrectIdolIds, idol.ID)
	}

	// create a bias game entry
	nugugameEntry := models.NuguGameEntry{
		ID:                  "",
		UserID:              game.User.ID,
		GuildID:             guild.ID,
		CorrectIdols:        correctIdolIds,
		IncorrectIdols:      incorrectIdolIds,
		GameType:            game.GameType,
		Gender:              game.Gender,
		Difficulty:          game.Difficulty,
		UsersCorrectGuesses: game.UsersCorrectGuesses,
	}

	log().Infoln(spew.Sdump(nugugameEntry))

	helpers.MDbInsert(models.NuguGameTable, nugugameEntry)
}
