package nugugame

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"

	"github.com/Seklfreak/Robyul2/modules/plugins/idols"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"
)

// recordNuguGame saves the nugugame to mongo
func recordNuguGame(g *nuguGame) {
	defer helpers.Recover() // this func is called via goroutine

	// small changes could be made to the game object during this func, make
	// copy so real game object isn't affected
	var game nuguGame
	game = *g

	// if the game doesn't have any correct guesses then ignore it. don't need
	// tons of games people didn't play in the db
	if len(game.CorrectIdols) == 0 {
		return
	}

	// get guildID from game channel
	channel, err := helpers.GetChannel(game.ChannelID)
	if err != nil {
		fmt.Println("Error getting channel when recording stats")
		return
	}
	guild, err := helpers.GetGuild(channel.GuildID)
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

	// if this game was a multi game but only one person played, change this to a solo game for that person who played
	gameUserId := game.User.ID
	if len(game.UsersCorrectGuesses) == 1 {
		game.IsMultigame = false
		for userId, _ := range game.UsersCorrectGuesses {
			gameUserId = userId
		}
		delete(game.UsersCorrectGuesses, gameUserId)
	}

	// create a nugugame entry
	nugugameEntry := models.NuguGameEntry{
		ID:                  "",
		UserID:              gameUserId,
		GuildID:             guild.ID,
		CorrectIdols:        correctIdolIds,
		CorrectIdolsCount:   len(correctIdolIds),
		IncorrectIdols:      incorrectIdolIds,
		IncorrectIdolsCount: len(incorrectIdolIds),
		GameType:            game.GameType,
		Gender:              game.Gender,
		Difficulty:          game.Difficulty,
		UsersCorrectGuesses: game.UsersCorrectGuesses,
		IsMultigame:         game.IsMultigame,
	}
	helpers.MDbInsert(models.NuguGameTable, nugugameEntry)
}

// displayNuguGameStats shows nugugame stats based on the users parameters
func displayNuguGameStats(msg *discordgo.Message, commandArgs []string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// strip out "stats" arg
	commandArgs = commandArgs[1:]

	targetUser := msg.Author

	// if there is only one arg check if it matches a valid group, if so send to group stats
	if len(commandArgs) == 1 {
		// if exists, _ := idols.GetMatchingGroup(commandArgs[1], true); exists {
		// 	displayGroupStats(msg, statsMessage)
		// 	return
		// }
	} else if len(commandArgs) == 2 {
		if _, _, idol := idols.GetMatchingIdolAndGroup(commandArgs[0], commandArgs[1], true); idol != nil {
			displayIdolStats(msg, commandArgs, idol)
			return
		}
	}

	// default
	query := bson.M{"ismultigame": false, "userid": targetUser.ID}
	isServerQuery := false
	isGlobalQuery := false
	var targetGuild *discordgo.Guild

	// check arguments
	var err error
	if len(commandArgs) > 0 {
		for _, arg := range commandArgs {

			if user, err := helpers.GetUserFromMention(arg); err == nil {
				if _, ok := query["userid"]; ok {
					query["userid"] = user.ID
					targetUser = user
				}
				continue
			}

			// check if running stats by server, default to the server of the message
			if arg == "server" {
				targetGuild, err = helpers.GetGuild(msg.GuildID)
				query = bson.M{"guildid": msg.GuildID}
				isServerQuery = true
				continue
			}

			// If stats are for a server, check if they also a serverid so we can run for other servers
			if isServerQuery {
				if targetGuild, err = helpers.GetGuild(commandArgs[len(commandArgs)-1]); err == nil {
					query = bson.M{"guildid": targetGuild.ID}
					continue
				}
			}

			// check if running stats globally, overrides server if both are included for some reason
			if arg == "global" {
				targetGuild = nil
				query = bson.M{}

				isServerQuery = false
				isGlobalQuery = true
				continue
			}

			// if a arg was passed that didn't match any check, send invalid args message
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
	}

	var games []models.NuguGameEntry
	helpers.MDbIter(helpers.MdbCollection(models.NuguGameTable).Find(query)).All(&games)

	highestScores := map[string]string{
		"overall": "*No Stats*",
		"easy":    "*No Stats*",
		"medium":  "*No Stats*",
		"hard":    "*No Stats*",
		"girl":    "*No Stats*",
		"boy":     "*No Stats*",
		"mixed":   "*No Stats*",
		"group":   "*No Stats*",
		"idol":    "*No Stats*",
		"multi":   "*No Stats*",
	}

	mostMissedIdols := make(map[*idols.Idol]int)
	mostMissedGroups := make(map[string]int)
	var totalCorrectCount int
	var totalIncorrectCount int

	// compile stats
	for _, game := range games {
		gameScore := game.CorrectIdolsCount

		totalCorrectCount += game.CorrectIdolsCount
		totalIncorrectCount += game.IncorrectIdolsCount

		// overall highest score
		curHighestEverything, _ := strconv.Atoi(highestScores["overall"])
		if gameScore > curHighestEverything {
			highestScores["overall"] = strconv.Itoa(gameScore)
		}

		// highest scores by difficulty
		curHighestByDifficulty, _ := strconv.Atoi(highestScores[game.Difficulty])
		if gameScore > curHighestByDifficulty {
			highestScores[game.Difficulty] = strconv.Itoa(gameScore)
		}

		// highest scores by gender
		curHighestByGender, _ := strconv.Atoi(highestScores[game.Gender])
		if gameScore > curHighestByGender {
			highestScores[game.Gender] = strconv.Itoa(gameScore)
		}

		// highest scores by game type
		curHighestByGametype, _ := strconv.Atoi(highestScores[game.GameType])
		if gameScore > curHighestByGametype {
			highestScores[game.GameType] = strconv.Itoa(gameScore)
		}

		// highest score for multi game
		if game.IsMultigame {
			curHighestForMulti, _ := strconv.Atoi(highestScores["multi"])
			if gameScore > curHighestForMulti {
				highestScores["multi"] = strconv.Itoa(gameScore)
			}
		}

		// get missed idols and groups
		for _, idolId := range game.IncorrectIdols {
			idol := idols.GetMatchingIdolById(idolId)

			if idol != nil {
				mostMissedIdols[idol] += 1
				mostMissedGroups[idol.GroupName] += 1
			}
		}
	}

	// calculate guess perfentage
	var correctGuessPercentage float64
	totalGuesses := totalCorrectCount + totalIncorrectCount
	if totalGuesses > 0 {
		correctGuessPercentage = (float64(totalCorrectCount) / float64(totalGuesses)) * 100
	} else {
		correctGuessPercentage = 0
	}

	// get idol they get wrong the most
	mostMissedIdol := "*No Stats*"
	mostMissedGroup := "*No Stats*"
	var mostMissedIdolCount int
	var mostMissedGroupCount int

	for idol, missCount := range mostMissedIdols {
		if missCount > mostMissedIdolCount {
			mostMissedIdol = fmt.Sprintf("%s %s", idol.GroupName, idol.Name)
		}
	}
	for groupName, missCount := range mostMissedGroups {
		if missCount > mostMissedGroupCount {
			mostMissedGroup = groupName
		}
	}

	// get embed title and icon
	var embedTitle string
	var embedIcon string
	if isGlobalQuery {
		embedTitle = "Global - Nugu Game Stats"
		embedIcon = cache.GetSession().State.User.AvatarURL("512")

	} else if isServerQuery {
		embedTitle = "Server - Nugu Game Stats"
		embedIcon = discordgo.EndpointGuildIcon(targetGuild.ID, targetGuild.Icon)

	} else {
		embedTitle = fmt.Sprintf("%s - Nugu Game Stats", targetUser.Username)
		embedIcon = targetUser.AvatarURL("512")
	}

	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name:    embedTitle,
			IconURL: embedIcon,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Total Games Played",
				Value:  strconv.Itoa(len(games)),
				Inline: true,
			},
			{
				Name:   "Highest Score",
				Value:  highestScores["overall"],
				Inline: true,
			},
			{
				Name:   "Average Score",
				Value:  strconv.Itoa(totalCorrectCount / len(games)),
				Inline: true,
			},
			{
				Name:   "Most Missed Idol",
				Value:  mostMissedIdol,
				Inline: true,
			},
			{
				Name:   "Most Missed Group",
				Value:  mostMissedGroup,
				Inline: true,
			},
			{
				Name:   "Correct Guess %",
				Value:  strconv.FormatFloat(correctGuessPercentage, 'f', 2, 64) + "%",
				Inline: true,
			},
			{
				Name:   "Highest Score (Easy)",
				Value:  highestScores["easy"],
				Inline: true,
			},
			{
				Name:   "Highest Score (Medium)",
				Value:  highestScores["medium"],
				Inline: true,
			},
			{
				Name:   "Highest Score (Hard)",
				Value:  highestScores["hard"],
				Inline: true,
			},
			{
				Name:   "Highest Score (Girl)",
				Value:  highestScores["girl"],
				Inline: true,
			},
			{
				Name:   "Highest Score (Boy)",
				Value:  highestScores["boy"],
				Inline: true,
			},
			{
				Name:   "Highest Score (Group)",
				Value:  highestScores["group"],
				Inline: true,
			},
		},
	}

	// add stats for server or global
	if isServerQuery || isGlobalQuery {
		embed.Fields = append(embed.Fields, []*discordgo.MessageEmbedField{
			{
				Name:   "Highest Score (Multi)",
				Value:  highestScores["multi"],
				Inline: true,
			},
		}...)
	}

	// add empty fields for better formatting
	if emptyFieldsToAdd := len(embed.Fields) % 3; emptyFieldsToAdd > 0 {
		emptyFieldsToAdd = 3 - emptyFieldsToAdd
		for i := 0; i < emptyFieldsToAdd; i++ {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   helpers.ZERO_WIDTH_SPACE,
				Value:  helpers.ZERO_WIDTH_SPACE,
				Inline: true,
			})
		}
	}

	helpers.SendEmbed(msg.ChannelID, embed)
}

// displayNugugameRanking sends embed of nugu game rankings
func displayNugugameRanking(msg *discordgo.Message, commandArgs []string, isServerRanking bool) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// strip out "ranking" arg
	commandArgs = commandArgs[1:]

	// default query
	query := bson.M{"ismultigame": false, "gametype": "idol"}

	var targetGuild *discordgo.Guild
	var isServerQuery bool
	var err error

	embedIcon := cache.GetSession().State.User.AvatarURL("512")
	embedTitle := "Nugu Game User Rankings"
	if isServerRanking {
		embedTitle = "Nugu Game Server Rankings"
	}

	// check arguments
	if len(commandArgs) > 0 {
		for _, arg := range commandArgs {

			// check if running stats by server, default to the server of the message
			if arg == "server" && !isServerRanking {
				targetGuild, err = helpers.GetGuild(msg.GuildID)
				query["guildid"] = msg.GuildID
				embedIcon = discordgo.EndpointGuildIcon(targetGuild.ID, targetGuild.Icon)
				isServerQuery = true
				continue
			}

			// If stats are for a server, check if they also a serverid so we can run for other servers
			if isServerQuery && !isServerRanking {
				if targetGuild, err = helpers.GetGuild(commandArgs[len(commandArgs)-1]); err == nil {
					query["guildid"] = targetGuild.ID
					embedIcon = discordgo.EndpointGuildIcon(targetGuild.ID, targetGuild.Icon)
					continue
				}
			}

			// check difficulty
			if _, ok := difficultyLives[arg]; ok {
				query["difficulty"] = arg
				continue
			}

			// if a arg was passed that didn't match any check, send invalid args message
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
	}

	// run query and check games were returend
	var games []models.NuguGameEntry
	helpers.MDbIter(helpers.MdbCollection(models.NuguGameTable).Find(query)).All(&games)
	if len(games) == 0 {
		helpers.SendMessage(msg.ChannelID, "No rankings found")
		return
	}

	// sort games by score
	sort.Slice(games, func(i, j int) bool {
		return games[i].CorrectIdolsCount > games[j].CorrectIdolsCount
	})

	// get top 50 unique users or servers
	highestScoreGamesMap := make(map[string]models.NuguGameEntry)
	var highestScoreGames []models.NuguGameEntry
	for _, game := range games {
		if isServerRanking {

			if _, ok := highestScoreGamesMap[game.GuildID]; !ok {
				highestScoreGamesMap[game.GuildID] = game
				highestScoreGames = append(highestScoreGames, game)
			}
		} else {

			if _, ok := highestScoreGamesMap[game.UserID]; !ok {
				highestScoreGamesMap[game.UserID] = game
				highestScoreGames = append(highestScoreGames, game)
			}
		}
		if len(highestScoreGamesMap) >= 50 {
			break
		}
	}

	// create embed
	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name:    embedTitle,
			IconURL: embedIcon,
		},
	}

	// add rankings fields
	for i, game := range highestScoreGames {

		// get display name of user or guilds
		displayName := "*Unknown*"
		if isServerRanking {

			guild, err := helpers.GetGuild(game.GuildID)
			if err == nil {
				displayName = guild.Name
			}

		} else {

			user, err := helpers.GetUser(game.UserID)
			if err == nil {
				displayName = user.Username
			}
		}

		if len(displayName) > 25 {
			displayName = displayName[0:25] + "..."
		}

		embed.Fields = append(embed.Fields, []*discordgo.MessageEmbedField{
			{
				Name:   fmt.Sprintf("Rank #%d", i+1),
				Value:  displayName,
				Inline: true,
			},
			{
				Name:   "Highest Score",
				Value:  humanize.Comma(int64(game.CorrectIdolsCount)),
				Inline: true,
			},
			{
				Name:   "Game Difficulty",
				Value:  game.Difficulty,
				Inline: true,
			},
		}...)
	}

	helpers.SendPagedMessage(msg, embed, 30)
}

// displayMissedIdols will display the most
func displayMissedIdols(msg *discordgo.Message, commandArgs []string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// strip out "missed" arg
	commandArgs = commandArgs[1:]

	targetUser := msg.Author

	// default
	query := bson.M{"ismultigame": false, "userid": targetUser.ID}
	isServerQuery := false
	isGlobalQuery := false
	var targetGuild *discordgo.Guild

	// check arguments
	var err error
	if len(commandArgs) > 0 {
		for _, arg := range commandArgs {

			if user, err := helpers.GetUserFromMention(arg); err == nil {
				if _, ok := query["userid"]; ok {
					targetUser = user
					query["userid"] = user.ID
				}
				continue
			}

			// check if running stats by server, default to the server of the message
			if arg == "server" {
				targetGuild, err = helpers.GetGuild(msg.GuildID)
				query = bson.M{"guildid": msg.GuildID}
				isServerQuery = true
				continue
			}

			// If stats are for a server, check if they also a serverid so we can run for other servers
			if isServerQuery {
				if targetGuild, err = helpers.GetGuild(commandArgs[len(commandArgs)-1]); err == nil {
					query = bson.M{"guildid": targetGuild.ID}
					continue
				}
			}

			// check if running stats globally, overrides server if both are included for some reason
			if arg == "global" {
				targetGuild = nil
				query = bson.M{}

				isServerQuery = false
				isGlobalQuery = true
				continue
			}

			// if a arg was passed that didn't match any check, send invalid args message
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
	}

	var games []models.NuguGameEntry
	helpers.MDbIter(helpers.MdbCollection(models.NuguGameTable).Find(query)).All(&games)

	mostMissedIdols := make(map[*idols.Idol]int)

	// compile stats
	for _, game := range games {

		// get missed idols and groups
		for _, idolId := range game.IncorrectIdols {
			idol := idols.GetMatchingIdolById(idolId)

			if idol != nil {
				mostMissedIdols[idol] += 1
			}
		}
	}

	// check if no idols have been missed
	if len(mostMissedIdols) == 0 {
		helpers.SendMessage(msg.ChannelID, "No missed idols found")
		return
	}

	// use map of counts to compile a new map of [unique occurence amounts][]idols
	var uniqueCounts []int
	compiledData := make(map[int][]string)
	for k, v := range mostMissedIdols {
		// store unique counts so the map can be "sorted"
		if _, ok := compiledData[v]; !ok {
			uniqueCounts = append(uniqueCounts, v)
		}

		compiledData[v] = append(compiledData[v], fmt.Sprintf("**%s** %s", k.GroupName, k.Name))
	}

	// sort biggest to smallest
	sort.Sort(sort.Reverse(sort.IntSlice(uniqueCounts)))

	// get embed title and icon
	var embedTitle string
	var embedIcon string
	if isGlobalQuery {
		embedTitle = "Global - Most Missed Idols"
		embedIcon = cache.GetSession().State.User.AvatarURL("512")

	} else if isServerQuery {
		embedTitle = "Server - Most Missed Idols"
		embedIcon = discordgo.EndpointGuildIcon(targetGuild.ID, targetGuild.Icon)

	} else {
		embedTitle = fmt.Sprintf("%s - Most Missed Idols", targetUser.Username)
		embedIcon = targetUser.AvatarURL("512")
	}

	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name:    embedTitle,
			IconURL: embedIcon,
		},
	}

	countLabel := "Missed Guesses"

	// loop through all the idols by most missed first
	for _, count := range uniqueCounts {

		// sort idols by group
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

	helpers.SendPagedMessage(msg, embed, 10)
}

// displayIdolStats displays nugugame stats for a given idol
func displayIdolStats(msg *discordgo.Message, commandArgs []string, targetIdol *idols.Idol) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// if an idol as passed, skip checking args
	if targetIdol == nil {

		// strip out "idol-stats" arg
		commandArgs = commandArgs[1:]

		if len(commandArgs) < 2 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
			return
		}

		// attempt to get matching idol
		_, _, targetIdol = idols.GetMatchingIdolAndGroup(commandArgs[0], commandArgs[1], true)
		if targetIdol == nil {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-matching-idol"))
			return
		}
	}

	// query games where idol is in game
	query := bson.M{"$or": []bson.M{
		// check if idol is in round winner or losers array
		bson.M{"correctidols": targetIdol.ID},
		bson.M{"incorrectidols": targetIdol.ID},
	}}

	// exclude unneeded fields for better performance
	fieldsToExclude := map[string]int{
		"correctidols":       0,
		"usercorrectguesses": 0,
	}

	var games []models.NuguGameEntry
	helpers.MDbIter(helpers.MdbCollection(models.NuguGameTable).Find(query).Select(fieldsToExclude)).All(&games)

	var totalCorrectGuesses int
	var totalIncorrectGuesses int

	// collect stats from games
	for _, game := range games {

		// check if idol was in correct guesses, if not add to correct guesses
		isIncorrectGuess := false
		for _, idolId := range game.IncorrectIdols {
			if targetIdol.ID == idolId {
				isIncorrectGuess = true
				totalIncorrectGuesses += 1
			}
		}

		if !isIncorrectGuess {
			totalCorrectGuesses += 1
		}
	}

	// calculate guess percentage
	var correctGuessPercentage float64
	totalGuesses := totalCorrectGuesses + totalIncorrectGuesses
	if totalGuesses > 0 {
		correctGuessPercentage = (float64(totalCorrectGuesses) / float64(totalGuesses)) * 100
	} else {
		correctGuessPercentage = 0
	}

	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("Stats for %s %s", targetIdol.GroupName, targetIdol.Name),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "attachment://idol_stats_thumbnail.png",
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Total Games",
				Value:  strconv.Itoa(totalCorrectGuesses + totalIncorrectGuesses),
				Inline: true,
			},
			{
				Name:   "Correct Guess %",
				Value:  strconv.FormatFloat(correctGuessPercentage, 'f', 2, 64) + "%",
				Inline: true,
			},
			{
				Name:   "Total Correct Guesses",
				Value:  strconv.Itoa(totalCorrectGuesses),
				Inline: true,
			},
			{
				Name:   "Total Incorrect Guesses",
				Value:  strconv.Itoa(totalIncorrectGuesses),
				Inline: true,
			},
		},
	}

	// get random image for the thumbnail
	imageIndex := rand.Intn(len(targetIdol.Images))
	thumbnailReader := bytes.NewReader(targetIdol.Images[imageIndex].GetImgBytes())

	msgSend := &discordgo.MessageSend{
		Files: []*discordgo.File{{
			Name:   "idol_stats_thumbnail.png",
			Reader: thumbnailReader,
		}},
		Embed: embed,
	}
	helpers.SendComplex(msg.ChannelID, msgSend)
}
