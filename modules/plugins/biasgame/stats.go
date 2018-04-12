package biasgame

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
	"github.com/mgutz/str"
)

type rankingStruct struct {
	userId           string
	guildId          string
	amountOfGames    int
	idolWithMostWins string
	userName         string
}

// displayBiasGameStats will display stats for the bias game based on the stats message
func displayBiasGameStats(msg *discordgo.Message, statsMessage string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	queryParams, iconURL, targetName := getStatsQueryInfo(msg, statsMessage)

	// check if round information is required
	fieldsToExclude := make(map[string]int)
	if !strings.Contains(statsMessage, "rounds") {
		fieldsToExclude = map[string]int{
			"roundwinners": 0,
			"roundlosers":  0,
		}
	}

	var games []models.BiasGameEntry
	helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(queryParams).Select(fieldsToExclude)).All(&games)

	// check if any stats were returned
	totalGames := len(games)
	if totalGames == 0 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-stats"))
		return
	}

	statsTitle := ""
	countsHeader := ""

	// loop through the results and compile a map of [biasgroup biasname]number of occurences
	biasCounts := make(map[string]int)
	for _, game := range games {
		groupAndName := ""

		if strings.Contains(statsMessage, "rounds won") {

			// round winners
			for _, rWinner := range game.RoundWinners {

				if strings.Contains(statsMessage, "group") {
					statsTitle = "Rounds Won in Bias Game by Group"
					groupAndName = fmt.Sprintf("%s", rWinner.GroupName)
				} else {
					statsTitle = "Rounds Won in Bias Game"
					groupAndName = fmt.Sprintf("**%s** %s", rWinner.GroupName, rWinner.Name)
				}
				biasCounts[groupAndName] += 1
			}

			countsHeader = "Rounds Won"

		} else if strings.Contains(statsMessage, "rounds lost") {

			// round losers
			for _, rLoser := range game.RoundLosers {

				if strings.Contains(statsMessage, "group") {
					statsTitle = "Rounds Lost in Bias Game by Group"
					groupAndName = fmt.Sprintf("%s", rLoser.GroupName)
				} else {
					statsTitle = "Rounds Lost in Bias Game"
					groupAndName = fmt.Sprintf("**%s** %s", rLoser.GroupName, rLoser.Name)
				}
				biasCounts[groupAndName] += 1
			}

			statsTitle = "Rounds Lost in Bias Game"
			countsHeader = "Rounds Lost"
		} else {

			// game winners
			if strings.Contains(statsMessage, "group") {
				statsTitle = "Bias Game Winners by Group"
				groupAndName = fmt.Sprintf("%s", game.GameWinner.GroupName)
			} else {
				statsTitle = "Bias Game Winners"
				groupAndName = fmt.Sprintf("**%s** %s", game.GameWinner.GroupName, game.GameWinner.Name)
			}

			biasCounts[groupAndName] += 1
			countsHeader = "Games Won"

		}
	}

	// add total games to the stats header message
	statsTitle = fmt.Sprintf("%s (%s games)", statsTitle, humanize.Comma(int64(totalGames)))

	sendStatsMessage(msg, statsTitle, countsHeader, biasCounts, iconURL, targetName)
}

// listIdolsInGame will list all idols that can show up in the biasgame
func listIdolsInGame(msg *discordgo.Message) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	genderCountMap := make(map[string]int)

	// create map of idols and there group
	groupIdolMap := make(map[string][]string)
	for _, bias := range getAllBiases() {
		genderCountMap[bias.Gender]++

		if len(bias.BiasImages) > 1 {

			groupIdolMap[bias.GroupName] = append(groupIdolMap[bias.GroupName], fmt.Sprintf("%s (%s)",
				bias.BiasName, humanize.Comma(int64(len(bias.BiasImages)))))
		} else {

			groupIdolMap[bias.GroupName] = append(groupIdolMap[bias.GroupName], fmt.Sprintf("%s", bias.BiasName))
		}
	}

	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("All Idols Available In Bias Game (%s total, %s girls, %s boys)",
				humanize.Comma(int64(len(getAllBiases()))),
				humanize.Comma(int64(genderCountMap["girl"])), humanize.Comma(int64(genderCountMap["boy"]))),
		},
		Title: "*Numbers indicate multi pictures are available for the idol*",
	}

	// make fields for each group and the idols in the group.
	for group, idols := range groupIdolMap {

		// sort idols by name
		sort.Slice(idols, func(i, j int) bool {
			return idols[i] < idols[j]
		})

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   group,
			Value:  strings.Join(idols, ", "),
			Inline: false,
		})
	}

	// sort fields by group name
	sort.Slice(embed.Fields, func(i, j int) bool {
		return strings.ToLower(embed.Fields[i].Name) < strings.ToLower(embed.Fields[j].Name)
	})

	helpers.SendPagedMessage(msg, embed, 10)
}

// showImagesForIdol will show a embed message with all the available images for an idol
func showImagesForIdol(msg *discordgo.Message, msgContent string, showObjectNames bool) {
	defer helpers.Recover()
	cache.GetSession().ChannelTyping(msg.ChannelID)

	commandArgs := str.ToArgv(msgContent)[1:]
	if len(commandArgs) < 2 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	// get matching idol to the group and name entered
	//  if we can't get one display an error
	groupMatch, nameMatch, matchIdol := getMatchingIdolAndGroup(commandArgs[0], commandArgs[1])
	if matchIdol == nil || groupMatch == false || nameMatch == false {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-matching-idol"))
		return
	}

	// get bytes of all the images
	var idolImages []biasImage
	for _, bImag := range matchIdol.BiasImages {
		idolImages = append(idolImages, bImag)
	}

	sendPagedEmbedOfImages(msg, idolImages, showObjectNames,
		fmt.Sprintf("Images for %s %s", matchIdol.GroupName, matchIdol.BiasName),
		fmt.Sprintf("Total Images: %s", humanize.Comma(int64(len(matchIdol.BiasImages)))))
}

// listIdolsInGame will list all idols that can show up in the biasgame
func showRankings(msg *discordgo.Message, commandArgs []string, isServerRanks bool) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	rankType := "user"
	gameType := "single"
	embedTitle := "Bias Game User Rankings"
	var filterGuild *discordgo.Guild

	// check if its server rankings
	if isServerRanks {
		rankType = "server"
		embedTitle = "Bias Game Server Rankings"
		gameType = "all"

		// check for game type
		if strings.Contains(msg.Content, "multi") {
			gameType = "multi"
			embedTitle = "Multi Bias Game Server Rankings"
		} else if strings.Contains(msg.Content, "single") {
			gameType = "single"
			embedTitle = "Single Bias Game Server Rankings"
		}

		// check if filtering user ranks by server
	} else if strings.Contains(msg.Content, "server") {

		// if last arg is a valid guild id, use that. otherwise get for current guild
		if guild, err := helpers.GetGuild(commandArgs[len(commandArgs)-1]); err == nil {

			filterGuild = guild
		} else {
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			guild, err := helpers.GetGuild(channel.GuildID)
			helpers.Relax(err)

			filterGuild = guild
		}
	}

	// exclude rounds from rankings query for better performance
	fieldsToExclude := map[string]int{
		"roundwinners": 0,
		"roundlosers":  0,
	}

	var games []models.BiasGameEntry
	if gameType == "all" {

		helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(bson.M{}).Select(fieldsToExclude)).All(&games)
	} else {

		helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(bson.M{"gametype": gameType}).Select(fieldsToExclude)).All(&games)
	}

	// check if any stats were returned
	totalGames := len(games)
	if totalGames == 0 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-stats"))
		return
	}

	// loop through the results and compile a map of userids => gameWinner group+name
	rankingsInfo := make(map[string][]string)
	for _, game := range games {
		if rankType == "user" {

			// check if filtering user ranks by server
			if filterGuild != nil && filterGuild.ID != game.GuildID {
				continue
			}

			rankingsInfo[game.UserID] = append(rankingsInfo[game.UserID], fmt.Sprintf("%s %s", game.GameWinner.GroupName, game.GameWinner.Name))
		} else {
			rankingsInfo[game.GuildID] = append(rankingsInfo[game.GuildID], fmt.Sprintf("%s %s", game.GameWinner.GroupName, game.GameWinner.Name))
		}
	}

	// get the amount of wins and idol with most wins for each user
	rankings := []*rankingStruct{}
	for rankTypeId, gameWinners := range rankingsInfo {
		rankInfo := &rankingStruct{
			amountOfGames: len(gameWinners),
		}
		if rankType == "user" {
			rankInfo.userId = rankTypeId
		} else {
			rankInfo.guildId = rankTypeId
		}

		// get idol with most wins for this user
		idolCountMap := make(map[string]int)
		highestWins := 0
		for _, idol := range gameWinners {
			idolCountMap[idol]++
		}
		for idol, amountOfGames := range idolCountMap {
			if amountOfGames > highestWins {
				highestWins = amountOfGames
				rankInfo.idolWithMostWins = idol
			}
		}

		rankings = append(rankings, rankInfo)
	}

	// sort rankings by most wins and get top 50
	// sort fields by group name
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].amountOfGames > rankings[j].amountOfGames
	})

	if len(rankings) > 35 {
		rankings = rankings[:35]
	}

	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name:    embedTitle,
			IconURL: cache.GetSession().State.User.AvatarURL("512"),
		},
	}

	if rankType == "user" && filterGuild != nil {
		embed.Author.Name = fmt.Sprintf("%s - %s\n", filterGuild.Name, embedTitle)
		embed.Author.IconURL = discordgo.EndpointGuildIcon(filterGuild.ID, filterGuild.Icon)
	}

	// make fields for each group and the idols in the group.
	for i, rankInfo := range rankings {

		displayName := "*Unknown*"
		if rankType == "user" {

			user, err := helpers.GetUser(rankInfo.userId)
			if err == nil {
				displayName = user.Username
			}
		} else {
			guildInfo, err := helpers.GetGuild(rankInfo.guildId)
			if err == nil {
				displayName = guildInfo.Name
			}
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("Rank #%d", i+1),
			Value:  displayName,
			Inline: true,
		})
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Total Games",
			Value:  humanize.Comma(int64(rankInfo.amountOfGames)),
			Inline: true,
		})
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Most Winning Idol",
			Value:  rankInfo.idolWithMostWins,
			Inline: true,
		})
	}

	helpers.SendPagedMessage(msg, embed, 21)
}

// displayCurrentGameStats will list the rounds and round winners of a currently running game
func displayCurrentGameStats(msg *discordgo.Message) {

	blankField := &discordgo.MessageEmbedField{
		Name:   ZERO_WIDTH_SPACE,
		Value:  ZERO_WIDTH_SPACE,
		Inline: true,
	}

	// find currently running game for the user or a mention if one exists
	userPlayingGame := msg.Author
	if user, err := helpers.GetUserFromMention(msg.Content); err == nil {
		userPlayingGame = user
	}

	if game := getSinglePlayerGameByUserID(userPlayingGame.ID); game != nil {

		embed := &discordgo.MessageEmbed{
			Color: 0x0FADED, // blueish
			Author: &discordgo.MessageEmbedAuthor{
				Name: fmt.Sprintf("%s - Current Game Info\n", userPlayingGame.Username),
			},
		}

		// for i := 0; i < len(game.RoundWinners); i++ {
		for i := len(game.RoundWinners) - 1; i >= 0; i-- {

			fieldName := fmt.Sprintf("Round %d:", i+1)
			if len(game.RoundWinners) == i+1 {
				fieldName = "Last Round:"
			}

			message := fmt.Sprintf("W: %s %s\nL: %s %s\n",
				game.RoundWinners[i].GroupName,
				game.RoundWinners[i].BiasName,
				game.RoundLosers[i].GroupName,
				game.RoundLosers[i].BiasName)

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fieldName,
				Value:  message,
				Inline: true,
			})
		}

		// notify user if no rounds have been played in the game yet
		if len(embed.Fields) == 0 {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "No Rounds",
				Value:  helpers.GetText("plugins.biasgame.current.no-rounds-played"),
				Inline: true,
			})
		}

		// this is to correct embed alignment
		if len(embed.Fields)%3 == 1 {
			embed.Fields = append(embed.Fields, blankField)
			embed.Fields = append(embed.Fields, blankField)
		} else if len(embed.Fields)%3 == 2 {
			embed.Fields = append(embed.Fields, blankField)
		}

		helpers.SendPagedMessage(msg, embed, 12)
	} else {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.current.no-running-game"))
	}
}

// recordSingleGamesStats will record the winner, round winners/losers, and other misc stats of a game
func recordSingleGamesStats(game *singleBiasGame) {

	// get guildID from game channel
	channel, _ := cache.GetSession().State.Channel(game.ChannelID)
	guild, err := cache.GetSession().State.Guild(channel.GuildID)
	if err != nil {
		fmt.Println("Error getting guild when recording stats")
		return
	}

	// create a bias game entry
	biasGameEntry := models.BiasGameEntry{
		ID:           "",
		UserID:       game.User.ID,
		GuildID:      guild.ID,
		GameType:     "single",
		Gender:       game.Gender,
		RoundWinners: compileGameWinnersLosers(game.RoundWinners),
		RoundLosers:  compileGameWinnersLosers(game.RoundLosers),
		GameWinner: models.BiasGameIdolEntry{
			Name:      game.GameWinnerBias.BiasName,
			GroupName: game.GameWinnerBias.GroupName,
			Gender:    game.GameWinnerBias.Gender,
		},
	}

	helpers.MDbInsert(models.BiasGameTable, biasGameEntry)
}

// recordSingleGamesStats will record the winner, round winners/losers, and other misc stats of a game
func recordMultiGamesStats(game *multiBiasGame) {

	// get guildID from game channel
	channel, _ := cache.GetSession().State.Channel(game.ChannelID)
	guild, err := cache.GetSession().State.Guild(channel.GuildID)
	if err != nil {
		fmt.Println("Error getting guild when recording stats")
		return
	}

	// create a bias game entry
	biasGameEntry := models.BiasGameEntry{
		ID:           "",
		GuildID:      guild.ID,
		GameType:     "multi",
		Gender:       game.Gender,
		RoundWinners: compileGameWinnersLosers(game.RoundWinners),
		RoundLosers:  compileGameWinnersLosers(game.RoundLosers),
		GameWinner: models.BiasGameIdolEntry{
			Name:      game.GameWinnerBias.BiasName,
			GroupName: game.GameWinnerBias.GroupName,
			Gender:    game.GameWinnerBias.Gender,
		},
	}

	helpers.MDbInsert(models.BiasGameTable, biasGameEntry)
}

// getStatsQueryInfo will get the stats results based on the stats message
func getStatsQueryInfo(msg *discordgo.Message, statsMessage string) (bson.M, string, string) {
	iconURL := ""
	targetName := ""
	channel, err := helpers.GetChannel(msg.ChannelID)
	helpers.Relax(err)
	guild, err := helpers.GetGuild(channel.GuildID)
	helpers.Relax(err)

	queryParams := bson.M{}

	// filter by game type. multi/single
	if strings.Contains(statsMessage, "multi") {
		queryParams["gametype"] = "multi"

		// multi stats games can run for server or global with server as the default
		if strings.Contains(statsMessage, "global") {

			iconURL = cache.GetSession().State.User.AvatarURL("512")
			targetName = "Global"
		} else {
			queryParams["guildid"] = guild.ID
			iconURL = discordgo.EndpointGuildIcon(guild.ID, guild.Icon)
			targetName = "Server"

		}
	} else {
		queryParams["gametype"] = "single"

		// user/server/global checks
		if strings.Contains(statsMessage, "server") {

			iconURL = discordgo.EndpointGuildIcon(guild.ID, guild.Icon)
			targetName = "Server"
			queryParams["guildid"] = guild.ID
		} else if strings.Contains(statsMessage, "global") {
			iconURL = cache.GetSession().State.User.AvatarURL("512")
			targetName = "Global"

		} else if user, err := helpers.GetUserFromMention(statsMessage); err == nil {

			iconURL = user.AvatarURL("512")
			targetName = user.Username
			queryParams["userid"] = user.ID

		} else {
			iconURL = msg.Author.AvatarURL("512")
			targetName = msg.Author.Username

			queryParams["userid"] = msg.Author.ID
		}

	}

	// filter by gamewinner gender
	if strings.Contains(statsMessage, "boy") || strings.Contains(statsMessage, "boys") {
		queryParams["gamewinner.gender"] = "boy"
	} else if strings.Contains(statsMessage, "girl") || strings.Contains(statsMessage, "girls") {
		queryParams["gamewinner.gender"] = "girl"
	}

	//  Note: not sure if want to do dates. might be kinda cool. but could cause confusion due to timezone issues
	// date checks
	// if strings.Contains(statsMessage, "today") {
	// 	// dateCheck := bson.NewObjectIdWithTime()
	// 	messageTime, _ := msg.Timestamp.Parse()

	// 	from := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, messageTime.Location())
	// 	to := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 0, messageTime.Location())

	// 	fromId := bson.NewObjectIdWithTime(from)
	// 	toId := bson.NewObjectIdWithTime(to)

	// 	queryParams["_id"] = bson.M{"$gte": fromId, "$lt": toId}
	// }

	return queryParams, iconURL, targetName
}

// complieGameStats will convert records from database into a:
// 		map[int number of occurentces]string group or biasnames comma delimited
// 		will also return []int of the sorted unique counts for reliable looping later
func complieGameStats(records map[string]int) (map[int][]string, []int) {

	// use map of counts to compile a new map of [unique occurence amounts]biasnames
	var uniqueCounts []int
	compiledData := make(map[int][]string)
	for k, v := range records {
		// store unique counts so the map can be "sorted"
		if _, ok := compiledData[v]; !ok {
			uniqueCounts = append(uniqueCounts, v)
		}

		compiledData[v] = append(compiledData[v], k)
	}

	// sort biggest to smallest
	sort.Sort(sort.Reverse(sort.IntSlice(uniqueCounts)))

	return compiledData, uniqueCounts
}

func sendStatsMessage(msg *discordgo.Message, title string, countLabel string, data map[string]int, iconURL string, targetName string) {

	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s - %s\n", targetName, title),
			IconURL: iconURL,
		},
	}

	// convert data to map[num of occurences]delimited biases
	compiledData, uniqueCounts := complieGameStats(data)
	for _, count := range uniqueCounts {

		// sort biases by group
		sort.Slice(compiledData[count], func(i, j int) bool {
			return compiledData[count][i] < compiledData[count][j]
		})

		joinedNames := strings.Join(compiledData[count], ", ")

		if len(joinedNames) < 1024 {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s - %s", countLabel, humanize.Comma(int64(count))),
				Value:  joinedNames,
				Inline: false,
			})

		} else {

			// for a specific count, split into multiple fields of at max 40 names
			dataForCount := compiledData[count]
			namesPerField := 40
			breaker := true
			for breaker {

				var namesForField string
				if len(dataForCount) >= namesPerField {
					namesForField = strings.Join(dataForCount[:namesPerField], ", ")
					dataForCount = dataForCount[namesPerField:]
				} else {
					namesForField = strings.Join(dataForCount, ", ")
					breaker = false
				}

				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   fmt.Sprintf("%s - %s", countLabel, humanize.Comma(int64(count))),
					Value:  namesForField,
					Inline: false,
				})

			}
		}

	}

	// send paged message with 5 fields per page
	helpers.SendPagedMessage(msg, embed, 5)
}

// compileGameWinnersLosers will loop through the biases and convert them to []models.BiasGameIdolEntry
func compileGameWinnersLosers(biases []*biasChoice) []models.BiasGameIdolEntry {

	var biasEntries []models.BiasGameIdolEntry
	for _, bias := range biases {
		biasEntries = append(biasEntries, models.BiasGameIdolEntry{
			Name:      bias.BiasName,
			GroupName: bias.GroupName,
			Gender:    bias.Gender,
		})
	}

	return biasEntries
}

// displayIdolStats sends an embed for stats on a specific idol
func displayIdolStats(msg *discordgo.Message, content string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// make sure there are enough arguments
	commandArgs := str.ToArgv(content)[1:]
	if len(commandArgs) < 2 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	// find matching idol
	_, _, bias := getMatchingIdolAndGroup(commandArgs[0], commandArgs[1])
	if bias == nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-matching-idol"))
		return
	}

	// get all the games that the target idol has been in
	queryParams := bson.M{"$or": []bson.M{
		// check if idol is in round winner or losers array
		bson.M{"roundwinners": bson.M{"$elemMatch": bson.M{"groupname": bias.GroupName, "name": bias.BiasName}}},
		bson.M{"roundlosers": bson.M{"$elemMatch": bson.M{"groupname": bias.GroupName, "name": bias.BiasName}}},
	}}

	// exclude rounds from rankings query for better performance
	fieldsToExclude := map[string]int{
		"roundwinners": 0,
		"roundlosers":  0,
	}

	// query db for information on this
	var allGames []models.BiasGameEntry
	var targetIdolGames []models.BiasGameEntry
	helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(bson.M{}).Select(fieldsToExclude)).All(&allGames)
	helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(queryParams)).All(&targetIdolGames)

	// get idol win counts
	allGamesIdolWinCounts := make(map[string]int)
	singleGamesIdolWinCounts := make(map[string]int)
	multiGamesIdolWinCounts := make(map[string]int)
	for _, game := range allGames {
		groupAndName := fmt.Sprintf("%s %s", game.GameWinner.GroupName, game.GameWinner.Name)
		allGamesIdolWinCounts[groupAndName] += 1
		if game.GameType == "multi" {
			multiGamesIdolWinCounts[groupAndName] += 1
		} else {
			singleGamesIdolWinCounts[groupAndName] += 1
		}
	}

	biasGroupName := fmt.Sprintf("%s %s", bias.GroupName, bias.BiasName)

	// get overall win rank
	compiledData, uniqueCounts := complieGameStats(allGamesIdolWinCounts)
	idolWinRank := 0
OverallWinLoop:
	for i, count := range uniqueCounts {
		for _, idolGroupName := range compiledData[count] {
			if idolGroupName == biasGroupName {
				idolWinRank = i + 1
				break OverallWinLoop
			}
		}
	}

	// get single win rank
	compiledData, uniqueCounts = complieGameStats(singleGamesIdolWinCounts)
	idolSingleWinRank := 0
SingleWinLoop:
	for i, count := range uniqueCounts {
		for _, idolGroupName := range compiledData[count] {
			if idolGroupName == biasGroupName {
				idolSingleWinRank = i + 1
				break SingleWinLoop
			}
		}
	}

	// get multi win rank
	compiledData, uniqueCounts = complieGameStats(multiGamesIdolWinCounts)
	idolMultiWinRank := 0
MultiWinLoop:
	for i, count := range uniqueCounts {
		for _, idolGroupName := range compiledData[count] {
			if idolGroupName == biasGroupName {
				idolMultiWinRank = i + 1
				break MultiWinLoop
			}
		}
	}

	totalGames := len(targetIdolGames)
	totalGameWins := 0
	totalRounds := 0
	totalRoundWins := 0
	userGameWinMap := make(map[string]int)
	guildGameWinMap := make(map[string]int)

	for _, game := range targetIdolGames {

		// win game
		if game.GameWinner.GroupName == bias.GroupName && game.GameWinner.Name == bias.BiasName {
			userGameWinMap[game.UserID]++
			guildGameWinMap[game.GuildID]++
			totalGameWins++
		}

		// round win
		for _, round := range game.RoundWinners {
			if round.GroupName == bias.GroupName && round.Name == bias.BiasName {
				totalRounds++
				totalRoundWins++
			}
		}
		// round lose
		for _, round := range game.RoundLosers {
			if round.GroupName == bias.GroupName && round.Name == bias.BiasName {
				totalRounds++
			}
		}
	}

	// get most winning user
	highestUserWins := 0
	var userId string
	for k, v := range userGameWinMap {
		if v > highestUserWins {
			highestUserWins = v
			userId = k
		}
	}
	userNameMostWins := "*Unknown*"
	userMostWins, err := helpers.GetUser(userId)
	if err == nil {
		userNameMostWins = userMostWins.Username
	}

	// get most winning server
	highestServerWins := 0
	var guildId string
	for k, v := range guildGameWinMap {
		if v > highestServerWins {
			highestServerWins = v
			guildId = k
		}
	}
	guildNameMostWins := "*Unknown*"
	guildMostWins, err := helpers.GetGuild(guildId)
	if err == nil {
		guildNameMostWins = guildMostWins.Name
	}

	// make embed
	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("Stats for %s %s", bias.GroupName, bias.BiasName),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "attachment://idol_stats_thumbnail.png",
		},
	}

	// overall game and game win info
	var gameWinPercentage float64
	if totalGames > 0 {
		gameWinPercentage = (float64(totalGameWins) / float64(totalGames)) * 100
	} else {
		gameWinPercentage = 0
	}

	// overall round and round win info
	var roundWinPercentage float64
	if totalGames > 0 {
		roundWinPercentage = (float64(totalRoundWins) / float64(totalRounds)) * 100
	} else {
		roundWinPercentage = 0
	}

	// add fields
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Overall Game Wins Rank", Value: fmt.Sprintf("Rank #%d", idolWinRank), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Pictures Available", Value: strconv.Itoa(len(bias.BiasImages)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Single Game Wins Rank", Value: fmt.Sprintf("Rank #%d", idolSingleWinRank), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Multi Game Wins Rank", Value: fmt.Sprintf("Rank #%d", idolMultiWinRank), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Games Won", Value: strconv.Itoa(totalGameWins), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Rounds Won", Value: strconv.Itoa(totalRoundWins), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Total Games", Value: strconv.Itoa(totalGames), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Total Rounds", Value: strconv.Itoa(totalRounds), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Game Win %", Value: strconv.FormatFloat(gameWinPercentage, 'f', 2, 64) + "%", Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Round Win %", Value: strconv.FormatFloat(roundWinPercentage, 'f', 2, 64) + "%", Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "User With Most Wins", Value: fmt.Sprintf("%s (%d wins)", userNameMostWins, highestUserWins), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Server With Most Wins", Value: fmt.Sprintf("%s (%d wins)", guildNameMostWins, highestServerWins), Inline: true})

	// get random image from the thumbnail
	imageIndex := rand.Intn(len(bias.BiasImages))
	thumbnailReader := bytes.NewReader(bias.BiasImages[imageIndex].getImgBytes())

	msgSend := &discordgo.MessageSend{
		Files: []*discordgo.File{{
			Name:   "idol_stats_thumbnail.png",
			Reader: thumbnailReader,
		}},
		Embed: embed,
	}
	helpers.SendComplex(msg.ChannelID, msgSend)
}
