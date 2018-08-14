package biasgame

import (
	"bytes"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
)

// displayBiasGameStats will display stats for the bias game based on the stats message
func displayBiasGameStats(msg *discordgo.Message, statsMessage string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// if there is only one arg check if it matches a valid group, if so send to group stats
	contentArg, _ := helpers.ToArgv(statsMessage)
	if len(contentArg) == 2 {
		if exists, _ := idols.GetMatchingGroup(contentArg[1], true); exists {
			displayGroupStats(msg, statsMessage)
			return
		}
	} else if len(contentArg) == 3 {
		if _, _, idol := idols.GetMatchingIdolAndGroup(contentArg[1], contentArg[2], true); idol != nil {
			displayIdolStats(msg, statsMessage)
			return
		}
	}

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

	// filter by gamewinner gender if needed
	var genderFilter string
	if strings.Contains(statsMessage, "boy") || strings.Contains(statsMessage, "boys") {
		genderFilter = "boy"
	} else if strings.Contains(statsMessage, "girl") || strings.Contains(statsMessage, "girls") {
		genderFilter = "girl"
	}
	if genderFilter != "" {
		for i := len(games) - 1; i >= 0; i-- {
			gameWinner := idols.GetMatchingIdolById(games[i].GameWinner)
			if gameWinner != nil && gameWinner.Gender != genderFilter {
				games = append(games[:i], games[i+1:]...)
			}
		}
	}

	// check if any stats were returned
	totalGames := len(games)
	if totalGames == 0 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-stats"))
		return
	}

	statsTitle := ""
	countsHeader := ""

	// loop through the results and compile a map of [biasgroup Name]number of occurences
	biasCounts := make(map[string]int)
	for _, game := range games {
		groupAndName := ""

		if strings.Contains(statsMessage, "rounds won") {

			// round winners
			for _, rWinner := range game.RoundWinners {

				rWinner := idols.GetMatchingIdolById(rWinner)
				if rWinner == nil {
					continue
				}

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

				rLoser := idols.GetMatchingIdolById(rLoser)
				if rLoser == nil {
					continue
				}

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

			gameWinner := idols.GetMatchingIdolById(game.GameWinner)
			if gameWinner == nil {
				continue
			}

			// game winners
			if strings.Contains(statsMessage, "group") {
				statsTitle = "Bias Game Winners by Group"
				groupAndName = fmt.Sprintf("%s", gameWinner.GroupName)
			} else {
				statsTitle = "Bias Game Winners"
				groupAndName = fmt.Sprintf("**%s** %s", gameWinner.GroupName, gameWinner.Name)
			}

			biasCounts[groupAndName] += 1
			countsHeader = "Games Won"

		}
	}

	// add total games to the stats header message
	statsTitle = fmt.Sprintf("%s (%s games)", statsTitle, humanize.Comma(int64(totalGames)))

	sendStatsMessage(msg, statsTitle, countsHeader, biasCounts, iconURL, targetName)
}

// showRankings will show the user rankings for biasgame
func showRankings(msg *discordgo.Message, commandArgs []string, isServerRanks bool) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	gameSizeFilter := "this.roundwinners.length > 0"
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

	// filter by game size if needed
	re := regexp.MustCompile("[0-9]+")
	if userEnteredNum, err := strconv.Atoi(re.FindString(msg.Content)); err == nil {
		if !allowedGameSizes[userEnteredNum] {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.game.invalid-game-size"))
			return
		}

		gameSizeFilter = fmt.Sprintf("this.roundwinners.length == %d", userEnteredNum-1)
	}

	// exclude rounds from rankings query for better performance
	fieldsToExclude := map[string]int{
		"roundwinners": 0,
		"roundlosers":  0,
	}

	var games []models.BiasGameEntry
	if gameType == "all" {

		helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(bson.M{"$where": gameSizeFilter}).Select(fieldsToExclude)).All(&games)
	} else {

		helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(bson.M{"gametype": gameType, "$where": gameSizeFilter}).Select(fieldsToExclude)).All(&games)
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
		gameWinner := idols.GetMatchingIdolById(game.GameWinner)
		if gameWinner == nil {
			continue
		}

		if rankType == "user" {

			// check if filtering user ranks by server
			if filterGuild != nil && filterGuild.ID != game.GuildID {
				continue
			}

			rankingsInfo[game.UserID] = append(rankingsInfo[game.UserID], fmt.Sprintf("%s %s", gameWinner.GroupName, gameWinner.Name))
		} else {
			rankingsInfo[game.GuildID] = append(rankingsInfo[game.GuildID], fmt.Sprintf("%s %s", gameWinner.GroupName, gameWinner.Name))
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

		if len(displayName) > 22 {
			displayName = displayName[0:22] + "..."
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
		Name:   helpers.ZERO_WIDTH_SPACE,
		Value:  helpers.ZERO_WIDTH_SPACE,
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
				game.RoundWinners[i].Name,
				game.RoundLosers[i].GroupName,
				game.RoundLosers[i].Name)

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
		GameWinner:   game.GameWinnerBias.ID,
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
		GameWinner:   game.GameWinnerBias.ID,
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

	return queryParams, iconURL, targetName
}

// complieGameStats will convert records from database into a:
// 		map[int number of occurentces]string group or Names comma delimited
// 		will also return []int of the sorted unique counts for reliable looping later
func complieGameStats(records map[string]int) (map[int][]string, []int) {

	// use map of counts to compile a new map of [unique occurence amounts]Names
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

// compileGameWinnersLosers will loop through idols and compile array of their ids
func compileGameWinnersLosers(idols []*idols.Idol) []bson.ObjectId {
	var idolIds []bson.ObjectId
	for _, idol := range idols {
		idolIds = append(idolIds, idol.ID)
	}
	return idolIds
}

// displayIdolStats sends an embed for stats on a specific idol
func displayIdolStats(msg *discordgo.Message, content string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	commandArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	commandArgs = commandArgs[1:]

	if len(commandArgs) < 2 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	// find matching idol
	_, _, targetIdol := idols.GetMatchingIdolAndGroup(commandArgs[0], commandArgs[1], true)
	if targetIdol == nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-matching-idol"))
		return
	}

	// get all the games that the target idol has been in
	queryParams := bson.M{"$or": []bson.M{
		// check if idol is in round winner or losers array
		bson.M{"roundwinners": targetIdol.ID},
		bson.M{"roundlosers": targetIdol.ID},
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
		gameWinner := idols.GetMatchingIdolById(game.GameWinner)
		if gameWinner == nil {
			continue
		}

		groupAndName := fmt.Sprintf("%s %s", gameWinner.GroupName, gameWinner.Name)
		allGamesIdolWinCounts[groupAndName] += 1
		if game.GameType == "multi" {
			multiGamesIdolWinCounts[groupAndName] += 1
		} else {
			singleGamesIdolWinCounts[groupAndName] += 1
		}
	}

	biasGroupName := fmt.Sprintf("%s %s", targetIdol.GroupName, targetIdol.Name)

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

		gameWinner := idols.GetMatchingIdolById(game.GameWinner)

		// win game
		if gameWinner != nil && gameWinner.GroupName == targetIdol.GroupName && gameWinner.Name == targetIdol.Name {
			userGameWinMap[game.UserID]++
			guildGameWinMap[game.GuildID]++
			totalGameWins++
		}

		// round win
		for _, roundWinnerId := range game.RoundWinners {
			roundWinner := idols.GetMatchingIdolById(roundWinnerId)
			if roundWinner != nil && roundWinner.GroupName == targetIdol.GroupName && roundWinner.Name == targetIdol.Name {
				totalRounds++
				totalRoundWins++
			}
		}
		// round lose
		for _, roundLoserId := range game.RoundLosers {
			roundLoser := idols.GetMatchingIdolById(roundLoserId)
			if roundLoser != nil && roundLoser.GroupName == targetIdol.GroupName && roundLoser.Name == targetIdol.Name {
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
			Name: fmt.Sprintf("Stats for %s %s", targetIdol.GroupName, targetIdol.Name),
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
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Pictures Available", Value: strconv.Itoa(len(targetIdol.Images)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Single Game Wins Rank", Value: fmt.Sprintf("Rank #%d", idolSingleWinRank), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Multi Game Wins Rank", Value: fmt.Sprintf("Rank #%d", idolMultiWinRank), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Games Won", Value: humanize.Comma(int64(totalGameWins)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Rounds Won", Value: humanize.Comma(int64(totalRoundWins)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Total Games", Value: humanize.Comma(int64(totalGames)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Total Rounds", Value: humanize.Comma(int64(totalRounds)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Game Win %", Value: strconv.FormatFloat(gameWinPercentage, 'f', 2, 64) + "%", Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Round Win %", Value: strconv.FormatFloat(roundWinPercentage, 'f', 2, 64) + "%", Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "User With Most Wins", Value: fmt.Sprintf("%s (%s wins)", userNameMostWins, humanize.Comma(int64(highestUserWins))), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Server With Most Wins", Value: fmt.Sprintf("%s (%s wins)", guildNameMostWins, humanize.Comma(int64(highestServerWins))), Inline: true})

	// get random image from the thumbnail
	imageIndex := rand.Intn(len(targetIdol.Images))
	thumbnailReader := bytes.NewReader(targetIdol.Images[imageIndex].GetResizeImgBytes(IMAGE_RESIZE_HEIGHT))

	msgSend := &discordgo.MessageSend{
		Files: []*discordgo.File{{
			Name:   "idol_stats_thumbnail.png",
			Reader: thumbnailReader,
		}},
		Embed: embed,
	}
	helpers.SendComplex(msg.ChannelID, msgSend)
}

// displayIdolStats sends an embed for stats on a specific idol
func displayGroupStats(msg *discordgo.Message, content string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	commandArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	commandArgs = commandArgs[1:]

	if len(commandArgs) < 1 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	// find matching idol
	groupMatched, targetGroupName := idols.GetMatchingGroup(commandArgs[0], false)
	if !groupMatched {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-matching-group"))
		return
	}

	// get all the games for the target group
	var orStatements []bson.M
	for _, idol := range idols.GetAllIdols() {
		if idol.GroupName == targetGroupName {
			orStatements = append(orStatements, []bson.M{
				bson.M{"roundwinners": idol.ID},
				bson.M{"roundlosers": idol.ID},
			}...)
		}
	}
	queryParams := bson.M{"$or": orStatements}

	// exclude rounds from rankings query for better performance
	fieldsToExclude := map[string]int{
		"roundwinners": 0,
		"roundlosers":  0,
	}

	// query db for information on this
	var allGames []models.BiasGameEntry
	var targetGroupGames []models.BiasGameEntry
	helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(bson.M{}).Select(fieldsToExclude)).All(&allGames)
	helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(queryParams)).All(&targetGroupGames)

	// get idol win counts
	allGamesGroupsWinCounts := make(map[string]int)
	singleGamesGroupWinCounts := make(map[string]int)
	multiGamesGroupWinCounts := make(map[string]int)
	memberWinCount := make(map[string]int)
	for _, game := range allGames {
		gameWinner := idols.GetMatchingIdolById(game.GameWinner)
		if gameWinner == nil {
			continue
		}

		if gameWinner.GroupName == targetGroupName {
			memberWinCount[gameWinner.Name] += 1
		}
		allGamesGroupsWinCounts[gameWinner.GroupName] += 1
		if game.GameType == "multi" {
			multiGamesGroupWinCounts[gameWinner.GroupName] += 1
		} else {
			singleGamesGroupWinCounts[gameWinner.GroupName] += 1
		}
	}

	// get overall win rank
	compiledData, uniqueCounts := complieGameStats(allGamesGroupsWinCounts)
	groupWinRank := 0
OverallWinLoop:
	for i, count := range uniqueCounts {
		for _, groupName := range compiledData[count] {
			if groupName == targetGroupName {
				groupWinRank = i + 1
				break OverallWinLoop
			}
		}
	}

	// get single win rank
	compiledData, uniqueCounts = complieGameStats(singleGamesGroupWinCounts)
	groupSingleWinRank := 0
SingleWinLoop:
	for i, count := range uniqueCounts {
		for _, groupName := range compiledData[count] {
			if groupName == targetGroupName {
				groupSingleWinRank = i + 1
				break SingleWinLoop
			}
		}
	}

	// get multi win rank
	compiledData, uniqueCounts = complieGameStats(multiGamesGroupWinCounts)
	groupMultiWinRank := 0
MultiWinLoop:
	for i, count := range uniqueCounts {
		for _, groupName := range compiledData[count] {
			if groupName == targetGroupName {
				groupMultiWinRank = i + 1
				break MultiWinLoop
			}
		}
	}

	totalGames := len(targetGroupGames)
	totalGameWins := 0
	totalRounds := 0
	totalRoundWins := 0
	userGameWinMap := make(map[string]int)
	guildGameWinMap := make(map[string]int)

	for _, game := range targetGroupGames {

		gameWinner := idols.GetMatchingIdolById(game.GameWinner)

		// win game
		if gameWinner != nil && gameWinner.GroupName == targetGroupName {
			userGameWinMap[game.UserID]++
			guildGameWinMap[game.GuildID]++
			totalGameWins++
		}

		// round win
		for _, round := range game.RoundWinners {
			roundWinner := idols.GetMatchingIdolById(round)
			if roundWinner != nil && roundWinner.GroupName == targetGroupName {
				totalRounds++
				totalRoundWins++
			}
		}

		// round lose
		for _, round := range game.RoundLosers {
			roundLoser := idols.GetMatchingIdolById(round)
			if roundLoser != nil && roundLoser.GroupName == targetGroupName {
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
			Name: fmt.Sprintf("Stats for %s", targetGroupName),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "attachment://group_stats_thumbnail.png",
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

	// get all images for the group
	var allGroupImages []idols.IdolImage
	for _, bias := range idols.GetActiveIdols() {
		if bias.GroupName != targetGroupName {
			continue
		}

		// get random picture for the idol
		imageIndex := rand.Intn(len(bias.Images))
		allGroupImages = append(allGroupImages, bias.Images[imageIndex])
	}

	// get group member with most wins
	var mostWinningMember string
	var mostWins int
	for name, winCount := range memberWinCount {
		if winCount > mostWins {
			mostWins = winCount
			mostWinningMember = name
		}

		// if win count is even between members, display first one alphabetically
		if winCount == mostWins && name > mostWinningMember {
			mostWinningMember = name
		}

	}

	// add fields
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Overall Game Wins Rank", Value: fmt.Sprintf("Rank #%d", groupWinRank), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Most Winning Member", Value: fmt.Sprintf("%s (%d wins)", mostWinningMember, mostWins), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Single Game Wins Rank", Value: fmt.Sprintf("Rank #%d", groupSingleWinRank), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Multi Game Wins Rank", Value: fmt.Sprintf("Rank #%d", groupMultiWinRank), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Games Won", Value: humanize.Comma(int64(totalGameWins)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Rounds Won", Value: humanize.Comma(int64(totalRoundWins)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Total Games", Value: humanize.Comma(int64(totalGames)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Total Rounds", Value: humanize.Comma(int64(totalRounds)), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Game Win %", Value: strconv.FormatFloat(gameWinPercentage, 'f', 2, 64) + "%", Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Round Win %", Value: strconv.FormatFloat(roundWinPercentage, 'f', 2, 64) + "%", Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "User With Most Wins", Value: fmt.Sprintf("%s (%s wins)", userNameMostWins, humanize.Comma(int64(highestUserWins))), Inline: true})
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Server With Most Wins", Value: fmt.Sprintf("%s (%s wins)", guildNameMostWins, humanize.Comma(int64(highestServerWins))), Inline: true})

	// get random image from the thumbnail
	imageIndex := rand.Intn(len(allGroupImages))
	thumbnailReader := bytes.NewReader(allGroupImages[imageIndex].GetImgBytes())

	msgSend := &discordgo.MessageSend{
		Files: []*discordgo.File{{
			Name:   "group_stats_thumbnail.png",
			Reader: thumbnailReader,
		}},
		Embed: embed,
	}
	helpers.SendComplex(msg.ChannelID, msgSend)

}

// updateGameStatsFromMsg will update saved game stats based on the discord message
func updateGameStatsFromMsg(msg *discordgo.Message, content string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	if !helpers.ConfirmEmbed(msg.ChannelID, msg.Author, "This command is still targeted at the old biasgame table, would you like to continue?", "✅", "🚫") {
		return
	}

	// validate arguments
	commandArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	commandArgs = commandArgs[1:]
	if len(commandArgs) != 5 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	if commandArgs[4] != "boy" && commandArgs[4] != "girl" {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	matched, modified := updateGameStats(commandArgs[0], commandArgs[1], commandArgs[2], commandArgs[3], commandArgs[4])

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Updated Stats. Changed %s %s => %s %s\nRecords Found: %s \nRecords Updated: %s", commandArgs[0], commandArgs[1], commandArgs[2], commandArgs[3], humanize.Comma(int64(matched)), humanize.Comma(int64(modified))))
}

// updateGameStats will update the stats based on the given parameters
//  returns the amount of records found and the amount updated
func updateGameStats(targetGroup, targetName, newGroup, newName, newGender string) (int, int) {

	// update is done in pairs, first the select query, and then the update.
	//  update gamewinner, roundwinner, and round loser records
	updateArray := []interface{}{
		bson.M{"gamewinner.groupname": targetGroup, "gamewinner.name": targetName},
		bson.M{"$set": bson.M{"gamewinner.groupname": newGroup, "gamewinner.name": newName, "gamewinner.gender": newGender}},

		bson.M{"roundwinners": bson.M{"$elemMatch": bson.M{"groupname": targetGroup, "name": targetName}}},
		bson.M{"$set": bson.M{"roundwinners.$.groupname": newGroup, "roundwinners.$.name": newName, "roundwinners.$.gender": newGender}},

		bson.M{"roundlosers": bson.M{"$elemMatch": bson.M{"groupname": targetGroup, "name": targetName}}},
		bson.M{"$set": bson.M{"roundlosers.$.groupname": newGroup, "roundlosers.$.name": newName, "roundlosers.$.gender": newGender}},
	}

	matched := 0
	modified := 0

	// update in a loop as $ is a positional operator and therefore not all array elements for the round will be updated immediatly. loop through and update them until completed
	//   wish this wasn't needed but mgo doesn't have a proper way to do arrayfilter with update multi mongo operation
	for true {

		// run bulk operation to update records
		bulkOperation := helpers.MdbCollection(models.OldBiasGameTable).Bulk()
		bulkOperation.UpdateAll(updateArray...)
		bulkResults, err := bulkOperation.Run()
		if err != nil {
			bgLog().Errorln("Bulk update error: ", err.Error())
			return 0, 0
		}

		matched += bulkResults.Matched
		modified += bulkResults.Modified

		// break when no more records are being modified
		if bulkResults.Modified == 0 {
			break
		}
	}

	return matched, modified
}

// validateStats will loop through all biasgames and confirm an idol exists for each game/round
func validateStats(msg *discordgo.Message, commandArgs []string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	helpers.SendMessage(msg.ChannelID, "Checking games for invalid idol ids...")

	find := helpers.MdbCollection(models.BiasGameTable).Find(bson.M{})

	game := models.BiasGameEntry{}
	games := find.Iter()

	missingIdolIds := make(map[bson.ObjectId]bool)

	for games.Next(&game) {
		gameWinner := idols.GetMatchingIdolById(game.GameWinner)

		if gameWinner == nil {
			missingIdolIds[game.GameWinner] = true
		}

		// round win
		for _, round := range game.RoundWinners {
			roundWinner := idols.GetMatchingIdolById(round)
			if roundWinner == nil {
				missingIdolIds[round] = true
			}
		}

		// round lose
		for _, round := range game.RoundLosers {
			roundLoser := idols.GetMatchingIdolById(round)
			if roundLoser == nil {
				missingIdolIds[round] = true
			}
		}
	}

	if len(missingIdolIds) > 0 {
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("%d idol ids in biasgames that don't match valid idols.", len(missingIdolIds)))

		if len(commandArgs) > 1 && commandArgs[1] == "print" {
			helpers.SendMessage(msg.ChannelID, "Printing ids:")
			var idsString string
			for id, _ := range missingIdolIds {
				idsString += id.String() + "\n"
			}

			helpers.SendMessage(msg.ChannelID, idsString)
		}

	} else {
		helpers.SendMessage(msg.ChannelID, "All biasgames have valid idols.")
	}
}
