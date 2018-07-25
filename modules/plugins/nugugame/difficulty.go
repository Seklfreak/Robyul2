package nugugame

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/globalsign/mgo/bson"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
)

const (
	NUGUGAME_DIFFICULTY_IDOLS_KEY = "nugugameIdolsByDifficulty"
)

var difficultyPercentageMap = map[string]float32{
	"easy":   .1,
	"medium": .3,
	"hard":   .7,
}

var idolsByDifficultyMutex sync.RWMutex
var idolsByDifficulty = map[string][]string{
	"easy":   []string{},
	"medium": []string{},
	"hard":   []string{},
}

// startDifficultyCacheLoop will refresh the cache for nugugame idols in difficulty
func startDifficultyCacheLoop() {
	log().Info("Starting nugugame cache loop")
	go func() {
		defer helpers.Recover()

		for {
			time.Sleep(time.Hour * 3)

			// refresh nugugame idols and save cache
			refreshDifficulties()
			log().Infof("Cached nugugame idols by difficulty")
		}
	}()
}

// getIdolsByDifficulty will return the objectID hexs of all idols for a certain difficulty of the nugugame
func getAllNugugameIdols() map[string][]string {
	idolsByDifficultyMutex.RLock()
	defer idolsByDifficultyMutex.RUnlock()
	return idolsByDifficulty
}

// getIdolsByDifficulty will return the objectID hexs of all idols for a certain difficulty of the nugugame
func getNugugameIdolsByDifficulty(difficulty string) []string {
	idolsByDifficultyMutex.RLock()
	defer idolsByDifficultyMutex.RUnlock()
	return idolsByDifficulty[difficulty]
}

// refreshDifficulties refreshes the idols included in each game difficulty
func refreshDifficulties() {
	log().Infoln("Refreshing nugugame idol difficulties...")

	// exclude rounds for faster querying
	fieldsToExclude := map[string]int{
		"roundwinners": 0,
		"roundlosers":  0,
	}

	var games []models.BiasGameEntry
	helpers.MDbIter(helpers.MdbCollection(models.BiasGameTable).Find(bson.M{}).Select(fieldsToExclude)).All(&games)

	// check if any stats were returned
	totalGames := len(games)
	if totalGames == 0 {
		log().Errorln("No biasgame returned when refreshing nugugame difficulties")
		return
	}

	// loop through the results and compile a map of [biasgroup Name]number of occurences
	winCounts := make(map[string]int)
	for _, game := range games {
		winCounts[game.GameWinner.Hex()] += 1
	}

	// convert data to map[num of occurences]delimited idols
	compiledData, uniqueCounts := complieGameStats(winCounts)
	var idolsInNugugame []string

	idolsByDifficultyMutex.Lock()
LoadDifficultyLoop:
	for _, count := range uniqueCounts {
		for _, name := range compiledData[count] {
			idolsInNugugame = append(idolsInNugugame, name)

			for difficulty, _ := range idolsByDifficulty {
				amountForDifficulty := float32(len(idols.GetActiveIdols())) * difficultyPercentageMap[difficulty]

				if len(idolsInNugugame) == int(amountForDifficulty) {
					idolsByDifficulty[difficulty] = idolsInNugugame

					if difficulty == "hard" {
						break LoadDifficultyLoop
					}
				}
			}
		}
	}
	idolsByDifficultyMutex.Unlock()

	log().Infof("Amount Of Idols By Difficulty: Easy: %d, Medium: %d, Hard: %d", len(idolsByDifficulty["easy"]), len(idolsByDifficulty["medium"]), len(idolsByDifficulty["hard"]))

	// update cache
	err := setModuleCache(NUGUGAME_DIFFICULTY_IDOLS_KEY, getAllNugugameIdols(), 0)
	helpers.Relax(err)
}

// listIdolsByDifficulty will display the idols included in a specific difficulty of the nugugame
func listIdolsByDifficulty(msg *discordgo.Message, commandArgs []string) {

	if len(commandArgs) < 2 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	var idolsInDifficulty []string
	var ok bool
	difficulty := strings.ToLower(commandArgs[1])
	if idolsInDifficulty, ok = idolsByDifficulty[difficulty]; !ok {
		helpers.SendMessage(msg.ChannelID, "That difficulty does not exist.")
		return
	}

	// loop through the results and compile a map of [biasgroup Name]number of occurences
	var allNames []string
	for _, idolID := range idolsInDifficulty {
		groupAndName := ""

		gameWinner := idols.GetMatchingIdolById(bson.ObjectIdHex(idolID))
		if gameWinner == nil {
			continue
		}
		groupAndName = fmt.Sprintf("**%s** %s", gameWinner.GroupName, gameWinner.Name)

		allNames = append(allNames, groupAndName)
	}

	// sort idols by group
	sort.Slice(allNames, func(i, j int) bool {
		return allNames[i] < allNames[j]
	})

	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("Idols included in %s difficulty (%d)", commandArgs[1], len(allNames)),
			IconURL: msg.Author.AvatarURL("512"),
		},
	}

	// for a specific count, split into multiple fields of at max 40 names
	namesPerField := 30
	breaker := true
	for breaker {

		var namesForField string
		if len(allNames) >= namesPerField {
			namesForField = strings.Join(allNames[:namesPerField], ", ")
			allNames = allNames[namesPerField:]
		} else {
			namesForField = strings.Join(allNames, ", ")
			breaker = false
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   helpers.ZERO_WIDTH_SPACE,
			Value:  namesForField,
			Inline: true,
		})

	}

	// send paged message with 5 fields per page
	helpers.SendPagedMessage(msg, embed, 5)
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

func manualRefreshDifficulties(msg *discordgo.Message) {
	helpers.SendMessage(msg.ChannelID, "Refreshing nugugame difficulties...")
	refreshDifficulties()
	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Amount Of Idols By Difficulty: \nEasy: %d \nMedium: %d \nHard: %d", len(idolsByDifficulty["easy"]), len(idolsByDifficulty["medium"]), len(idolsByDifficulty["hard"])))
}
