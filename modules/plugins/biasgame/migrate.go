package biasgame

import (
	"fmt"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
)

// runGameMigration loops through all games and all game rounds that don't already have an idol id
func runGameMigration(msg *discordgo.Message, content string) {

	// check if the new table already exists and has records. if it does block the migration again
	count, err := helpers.GetMDb().C(models.BiasGameTable.String()).Count()
	if err != nil {
		cache.GetLogger().Errorln(err.Error())
		return
	}
	// checking for greater than 10000 because we still want to allow games to be recorded while migration is running
	if count > 10000 {
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Migration has already been run. If something went wrong please drop table **%s** and run this again.", models.BiasGameTable))
		return
	}

	helpers.SendMessage(msg.ChannelID, "Loading all bias games info...")

	// get all biasgame entries
	var gameEntries []models.OldBiasGameEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.OldBiasGameTable).Find(bson.M{})).All(&gameEntries)
	helpers.Relax(err)

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Games found: %d", len(gameEntries)))
	helpers.SendMessage(msg.ChannelID, "Checking for idols in games that don't currently exist...")

	missingIdolNames, affectedGames := findBiasgamesWithoutExistingIdols(gameEntries)

	// if games were found then stop migration as they needed to be cleaned up before migration can continue
	if affectedGames > 0 {
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Missing idols found: %d\nTotal Games Affected: %d", len(missingIdolNames), affectedGames))
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Names: \n%s", strings.Join(missingIdolNames, "\n")))
		return
	} else {
		helpers.SendMessage(msg.ChannelID, "All games have valid idols, running migration...")
	}

	idolNameToId := make(map[string]bson.ObjectId)
	var newGameEntries []interface{}
	for _, gameEntry := range gameEntries {
		newGameEntry := models.BiasGameEntry{
			UserID:   gameEntry.UserID,
			GuildID:  gameEntry.GuildID,
			Gender:   gameEntry.Gender,
			GameType: gameEntry.GameType,
		}

		// check if this idol is saved in our map
		if objectId, ok := idolNameToId[gameEntry.GameWinner.GroupName+"="+gameEntry.GameWinner.Name]; ok {
			newGameEntry.GameWinner = objectId
		} else {
			_, _, idol := idols.GetMatchingIdolAndGroup(gameEntry.GameWinner.GroupName, gameEntry.GameWinner.Name, false)
			idolNameToId[gameEntry.GameWinner.GroupName+"="+gameEntry.GameWinner.Name] = idol.ID
			newGameEntry.GameWinner = idol.ID
		}

		for _, round := range gameEntry.RoundWinners {
			if objectId, ok := idolNameToId[round.GroupName+"="+round.Name]; ok {
				newGameEntry.RoundWinners = append(newGameEntry.RoundWinners, objectId)
			} else {
				_, _, idol := idols.GetMatchingIdolAndGroup(round.GroupName, round.Name, false)
				idolNameToId[round.GroupName+"="+round.Name] = idol.ID
				newGameEntry.RoundWinners = append(newGameEntry.RoundWinners, idol.ID)
			}
		}

		for _, round := range gameEntry.RoundLosers {
			if objectId, ok := idolNameToId[round.GroupName+"="+round.Name]; ok {
				newGameEntry.RoundLosers = append(newGameEntry.RoundLosers, objectId)
			} else {
				_, _, idol := idols.GetMatchingIdolAndGroup(round.GroupName, round.Name, false)
				idolNameToId[round.GroupName+"="+round.Name] = idol.ID
				newGameEntry.RoundLosers = append(newGameEntry.RoundLosers, idol.ID)
			}
		}

		newGameEntries = append(newGameEntries, newGameEntry)
	}

	bulkOperation := helpers.MdbCollection(models.BiasGameTable).Bulk()
	bulkOperation.Insert(newGameEntries...)
	_, err = bulkOperation.Run()
	if err != nil {
		bgLog().Errorln("Bulk update error: ", err.Error())
	}

	// delete any currently running games and clear the redis cache
	currentSinglePlayerGamesMutex.Lock()
	for k := range currentSinglePlayerGames {
		delete(currentSinglePlayerGames, k)
	}
	currentSinglePlayerGamesMutex.Unlock()
	currentMultiPlayerGamesMutex.Lock()
	currentMultiPlayerGames = nil
	currentMultiPlayerGamesMutex.Unlock()
	err = setBiasGameCache("currentSinglePlayerGames", getCurrentSinglePlayerGames(), 0)
	helpers.Relax(err)
	err = setBiasGameCache("currentMultiPlayerGames", getCurrentMultiPlayerGames(), 0)
	helpers.Relax(err)

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("New games: %d", len(newGameEntries)))
	helpers.SendMessage(msg.ChannelID, "Done.")
}

// will check the games passed to it so see if any of the games contain idols that don't exist
func findBiasgamesWithoutExistingIdols(gameEntries []models.OldBiasGameEntry) ([]string, int) {

	// get unique group and name combinations
	uniqueNames := make(map[string]bool)
	for _, game := range gameEntries {
		// using = as an easy to combine group and name while still being able to split later
		uniqueNames[game.GameWinner.GroupName+"="+game.GameWinner.Name] = true

		for _, round := range game.RoundWinners {
			uniqueNames[round.GroupName+"="+round.Name] = true
		}

		for _, round := range game.RoundLosers {
			uniqueNames[round.GroupName+"="+round.Name] = true
		}
	}

	// use unique name map to quickly check which idols don't exist
	gamesWithoutExistingWinner := make(map[string]bool)
	for groupAndName := range uniqueNames {
		splitGroupAndName := strings.Split(groupAndName, "=")
		_, _, idol := idols.GetMatchingIdolAndGroup(splitGroupAndName[0], splitGroupAndName[1], false)

		if idol == nil {
			gamesWithoutExistingWinner[splitGroupAndName[0]+" "+splitGroupAndName[1]] = true
		} else {
			delete(uniqueNames, groupAndName)
		}
	}

	// if any exist idols in games don't currently exist in the idols table then check how many games they're affecting
	var affectedGames []models.OldBiasGameEntry
GameLoop:
	for _, game := range gameEntries {

		if _, ok := uniqueNames[game.GameWinner.GroupName+"="+game.GameWinner.Name]; ok {
			affectedGames = append(affectedGames, game)
			continue GameLoop
		}

		for _, round := range game.RoundWinners {
			if _, ok := uniqueNames[round.GroupName+"="+round.Name]; ok {
				affectedGames = append(affectedGames, game)
				continue GameLoop
			}
		}

		for _, round := range game.RoundLosers {
			if _, ok := uniqueNames[round.GroupName+"="+round.Name]; ok {
				affectedGames = append(affectedGames, game)
				continue GameLoop
			}
		}
	}

	var gamesWithoutExistingWinnerAr []string
	for groupAndName := range gamesWithoutExistingWinner {
		gamesWithoutExistingWinnerAr = append(gamesWithoutExistingWinnerAr, groupAndName)
	}

	return gamesWithoutExistingWinnerAr, len(affectedGames)
}
