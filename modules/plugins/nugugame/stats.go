package nugugame

import (
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"github.com/Seklfreak/Robyul2/modules/plugins/idols"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"
)

// recordNuguGame saves the nugugame to mongo
func recordNuguGame(g *nuguGame) {
	defer helpers.Recover() // this func is called via coroutine

	// small changes could be made to the game object during this func, make
	// copy so real game object isn't affected
	var game nuguGame
	game = *g

	// if the game doesn't have any correct guesses then ignore it. don't need tons of games people didn't play in the db
	if len(game.CorrectIdols) == 0 {
		return
	}

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

	// if this game was a multi game but only one person played, change this to a solo game for that person who played
	gameUserId := game.User.ID
	if len(game.UsersCorrectGuesses) == 1 {
		game.IsMultigame = false
		for userId, _ := range game.UsersCorrectGuesses {
			gameUserId = userId
		}
		delete(game.UsersCorrectGuesses, gameUserId)
	}

	// create a bias game entry
	nugugameEntry := models.NuguGameEntry{
		ID:                  "",
		UserID:              gameUserId,
		GuildID:             guild.ID,
		CorrectIdols:        correctIdolIds,
		IncorrectIdols:      incorrectIdolIds,
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
	// strip out "stats" arg
	commandArgs = commandArgs[1:]

	targetUser := msg.Author
	if user, err := helpers.GetUserFromMention(msg.Content); err == nil {
		targetUser = user
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
		"all":     "*No Stats*",
		"girl":    "*No Stats*",
		"boy":     "*No Stats*",
		"mixed":   "*No Stats*",
		"group":   "*No Stats*",
		"idol":    "*No Stats*",
		"multi":   "*No Stats*",
	}

	mostMissedIdols := make(map[*idols.Idol]int)
	mostMissedGroups := make(map[string]int)

	// compile stats
	for _, game := range games {
		gameScore := len(game.CorrectIdols)

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
				Name:   "Highest Score (All)",
				Value:  highestScores["all"],
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

	// add stats for server
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
