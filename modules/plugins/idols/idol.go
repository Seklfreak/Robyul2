package idols

import (
	"fmt"
	"image"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
)

const (
	ALL_IDOLS_CACHE_KEY = "allidols"
)

// holds all available idols
var allIdols []*Idol
var allIdolsMutex sync.RWMutex

////////////////////
//  Idol Methods  //
////////////////////

// GetRandomImage returns a random idol image
func (i *Idol) GetRandomImage() image.Image {

	imageIndex := rand.Intn(len(i.BiasImages))
	imgBytes := i.BiasImages[imageIndex].GetImgBytes()
	img, _, err := helpers.DecodeImageBytes(imgBytes)
	helpers.Relax(err)
	return img
}

// GetResizedRandomImage returns a random image that has been resized
func (i *Idol) GetResizedRandomImage(resize int) image.Image {

	imageIndex := rand.Intn(len(i.BiasImages))
	imgBytes := i.BiasImages[imageIndex].GetResizeImgBytes(resize)
	img, _, err := helpers.DecodeImageBytes(imgBytes)
	helpers.Relax(err)
	return img
}

// GetAllIdols getter for all idols
func GetAllIdols() []*Idol {
	allIdolsMutex.RLock()
	defer allIdolsMutex.RUnlock()

	if allIdols == nil {
		return nil
	}

	return allIdols
}

////////////////////////
//  Public Functions  //
////////////////////////

// GetMatchingIdolAndGroup will do a loose comparison of the name and group passed to the ones that already exist
//  1st return is true if group exists
//  2nd return is true if idol exists in the group
//  3rd will be a reference to the matching idol
func GetMatchingIdolAndGroup(searchGroup, searchName string) (bool, bool, *Idol) {
	groupMatch := false
	nameMatch := false
	var matchingIdol *Idol

	// find a matching group
	groupMatch, realMatchingGroupName := GetMatchingGroup(searchGroup)

	// if no matching group was found, just return 0 values
	if !groupMatch {
		return false, false, nil
	}

	// find matching idol in the matching group
	for _, idol := range GetAllIdols() {

		if idol.GroupName != realMatchingGroupName {
			continue
		}

		if alphaNumericCompare(idol.BiasName, searchName) {
			nameMatch = true
			matchingIdol = idol
			break
		}
	}

	return groupMatch, nameMatch, matchingIdol
}

// getMatchingGroup will do a loose comparison of the group name to see if it exists
// return 1: if a matching group exists
// return 2: what the real group name is
func GetMatchingGroup(searchGroup string) (bool, string) {

	allGroupsMap := make(map[string]bool)
	for _, idol := range GetAllIdols() {
		allGroupsMap[idol.GroupName] = true
	}

	groupAliases := getGroupAliases()

	// check if the group suggested matches a current group. do loose comparison
	for currentGroup, _ := range allGroupsMap {

		// if groups match, set the suggested group to the current group
		if alphaNumericCompare(currentGroup, searchGroup) {
			return true, currentGroup
		}

		// if this group has any aliases check if the group we're
		//   searching for matches one of the aliases
		for aliasGroup, aliases := range groupAliases {
			if alphaNumericCompare(aliasGroup, currentGroup) {
				continue
			}

			for _, alias := range aliases {
				if alphaNumericCompare(alias, searchGroup) {
					return true, currentGroup
				}
			}
		}
	}

	return false, ""
}

/////////////////////////
//  Private Functions  //
/////////////////////////

// startCacheRefreshLoop will refresh the image cache for idols
func startCacheRefreshLoop() {
	log().Info("Starting biasgame refresh image cache loop")
	go func() {
		defer helpers.Recover()

		for {
			time.Sleep(time.Hour * 12)

			log().Info("Refreshing image cache...")
			refreshIdols(true)

			log().Info("Biasgame image cache has been refresh")
		}
	}()
}

// refreshIdols refreshes the idols
//   initially called when bot starts but is also safe to call while bot is running if necessary
func refreshIdols(skipCache bool) {

	if !skipCache {

		// attempt to get redis cache, return if its successful
		var tempAllIdols []*Idol
		err := getModuleCache(ALL_IDOLS_CACHE_KEY, &tempAllIdols)
		if err == nil {
			setAllIdols(tempAllIdols)
			log().Info("Idols loaded from cache")
			return
		}

		log().Info("Idols loading from mongodb. Cache not set or expired.")
	}

	var idolEntries []models.BiasGameIdolEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{})).All(&idolEntries)
	helpers.Relax(err)

	log().Infof("Loading idols. Total image records: %d", len(idolEntries))

	var tempAllIdols []*Idol

	// run limited amount of goroutines at the same time
	mux := new(sync.Mutex)
	sem := make(chan bool, 50)
	for _, idolEntry := range idolEntries {
		sem <- true
		go func(idolEntry models.BiasGameIdolEntry) {
			defer func() { <-sem }()
			defer helpers.Recover()

			newIdol := makeIdolFromIdolEntry(idolEntry)

			mux.Lock()
			defer mux.Unlock()

			// if the idol already exists, then just add this picture to the image array for the idol
			for _, currentIdol := range tempAllIdols {
				if currentIdol.NameAndGroup == newIdol.NameAndGroup {
					currentIdol.BiasImages = append(currentIdol.BiasImages, newIdol.BiasImages[0])
					return
				}
			}
			tempAllIdols = append(tempAllIdols, &newIdol)
		}(idolEntry)
	}
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}

	log().Info("Amount of idols loaded: ", len(tempAllIdols))
	setAllIdols(tempAllIdols)

	// cache all idols
	if len(GetAllIdols()) > 0 {
		err = setModuleCache(ALL_IDOLS_CACHE_KEY, GetAllIdols(), time.Hour*24*7)
		helpers.RelaxLog(err)
	}
}

// makeIdolFromIdolEntry takes a mdb idol entry and makes a idol
func makeIdolFromIdolEntry(entry models.BiasGameIdolEntry) Idol {
	iImage := IdolImage{
		ObjectName: entry.ObjectName,
	}

	// get image hash string
	img, _, err := helpers.DecodeImageBytes(iImage.GetImgBytes())
	helpers.Relax(err)
	imgHash, err := helpers.GetImageHashString(img)
	helpers.Relax(err)
	iImage.HashString = imgHash

	newIdol := Idol{
		BiasName:     entry.Name,
		GroupName:    entry.GroupName,
		Gender:       entry.Gender,
		NameAndGroup: entry.Name + entry.GroupName,
		BiasImages:   []IdolImage{iImage},
	}
	return newIdol
}

// updateGroupInfo if a target group is found, this will update the group name
//  for all members as well as updating all the stats for those members
func updateGroupInfo(msg *discordgo.Message, content string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	contentArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	contentArgs = contentArgs[1:]

	// confirm amount of args
	if len(contentArgs) != 2 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	targetGroup := contentArgs[0]
	newGroup := contentArgs[1]

	// confirm target group exists
	if matched, realGroupName := GetMatchingGroup(targetGroup); !matched {
		helpers.SendMessage(msg.ChannelID, "No group found with that exact name.")
		return
	} else {
		targetGroup = realGroupName
	}

	// update all idols in the target group
	var idolsUpdated int
	var allStatsUpdated int
	for _, idol := range GetAllIdols() {
		if idol.GroupName == targetGroup {

			recordsUpdated, _, statsUpdated := updateIdolInfo(idol.GroupName, idol.BiasName, newGroup, idol.BiasName, idol.Gender)
			if recordsUpdated != 0 {
				idolsUpdated++
				allStatsUpdated += statsUpdated
			}
			helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Updated Idol: **%s** %s => **%s** %s \nStats Updated: %s", targetGroup, idol.BiasName, newGroup, idol.BiasName, humanize.Comma(int64(statsUpdated))))

			// sleep so mongo doesn't get flooded with update reqeusts
			time.Sleep(time.Second / 5)
		}
	}

	// check if an idol record was updated
	if idolsUpdated == 0 {
		helpers.SendMessage(msg.ChannelID, "No Idols found in the given group.")
	} else {
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Group Information updated. \nIdols Updated: %d \nTotal Stats Updated: %s", idolsUpdated, humanize.Comma(int64(allStatsUpdated))))
	}
}

// updateIdolInfoFromMsg updates a idols group, name, and/or gender depending on args
func updateIdolInfoFromMsg(msg *discordgo.Message, content string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	contentArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	contentArgs = contentArgs[1:]

	// confirm amount of args
	if len(contentArgs) < 5 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	// validate gender
	if contentArgs[4] != "boy" && contentArgs[4] != "girl" {
		helpers.SendMessage(msg.ChannelID, "Invalid gender. Gender must be exactly 'girl' or 'boy'. No information was updated.")
		return
	}

	targetGroup := contentArgs[0]
	targetName := contentArgs[1]
	newGroup := contentArgs[2]
	newName := contentArgs[3]
	newGender := contentArgs[4]

	// update idol
	recordsUpdated, _, statsUpdated := updateIdolInfo(targetGroup, targetName, newGroup, newName, newGender)

	// check if an idol record was updated
	if recordsUpdated == 0 {
		helpers.SendMessage(msg.ChannelID, "No Idols found with that exact group and name.")
	} else {
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Idol Information updated. \nOld: **%s** %s \nNew: **%s** %s \nStats Updated: %d", targetGroup, targetName, newGroup, newName, statsUpdated))
	}
}

// updateIdolInfo updates a idols group, name, and/or gender depending on args
//  return 1: idol records updated
//  return 2: stats records found
//  return 3: stats records updated
func updateIdolInfo(targetGroup, targetName, newGroup, newName, newGender string) (int, int, int) {

	// attempt to find a matching idol of the new group and name,
	_, _, matchingIdol := GetMatchingIdolAndGroup(newGroup, newName)

	recordsFound := 0
	statsFound := 0
	statsUpdated := 0

	// update idols in memory
	allIdols := GetAllIdols()
	allIdolsMutex.Lock()
	for idolIndex, targetIdol := range allIdols {
		if targetIdol.BiasName != targetName || targetIdol.GroupName != targetGroup {
			continue
		}
		recordsFound++

		// if a matching idol was is found, just assign the targets images to it and delete
		if matchingIdol != nil && (matchingIdol.BiasName != targetIdol.BiasName || matchingIdol.GroupName != targetIdol.GroupName) {

			matchingIdol.BiasImages = append(matchingIdol.BiasImages, targetIdol.BiasImages...)
			allIdols = append(allIdols[:idolIndex], allIdols[idolIndex+1:]...)

			// update previous game stats
			// TODO
			// statsFound, statsUpdated = updateGameStats(targetIdol.GroupName, targetIdol.BiasName, matchingIdol.GroupName, matchingIdol.BiasName, matchingIdol.Gender)
			statsFound, statsUpdated = 0, 0

		} else {

			// update previous game stats
			// TODO
			// statsFound, statsUpdated = updateGameStats(targetIdol.GroupName, targetIdol.BiasName, newGroup, newName, newGender)
			statsFound, statsUpdated = 0, 0

			// update targetIdol name and group
			targetIdol.BiasName = newName
			targetIdol.GroupName = newGroup
			targetIdol.Gender = newGender
		}
	}
	allIdolsMutex.Unlock()
	setAllIdols(allIdols)

	// update database
	var idolsToUpdate []models.BiasGameIdolEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{"groupname": targetGroup, "name": targetName})).All(&idolsToUpdate)
	helpers.Relax(err)

	for _, idol := range idolsToUpdate {
		idol.Name = newName
		idol.GroupName = newGroup
		idol.Gender = newGender

		err := helpers.MDbUpsertID(models.BiasGameIdolsTable, idol.ID, idol)
		helpers.Relax(err)
	}

	// update cache
	if len(GetAllIdols()) > 0 {
		setModuleCache(ALL_IDOLS_CACHE_KEY, GetAllIdols(), time.Hour*24*7)
	}

	return recordsFound, statsFound, statsUpdated
}

// updateImageInfo updates a specific image and its related idol info
func updateImageInfo(msg *discordgo.Message, content string) {
	contentArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	contentArgs = contentArgs[1:]

	// confirm amount of args
	if len(contentArgs) < 4 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	targetObjectName := contentArgs[0]
	newGroup := contentArgs[1]
	newName := contentArgs[2]
	newGender := strings.ToLower(contentArgs[3])

	// if a gender was passed, make sure its valid
	if newGender != "boy" && newGender != "girl" {
		helpers.SendMessage(msg.ChannelID, "Invalid gender. Gender must be exactly 'girl' or 'boy'. No information was updated.")
		return
	}

	allIdols := GetAllIdols()
	allIdolsMutex.Lock()
	imageFound := false

	// find and delete target image by object name
IdolsLoop:
	for idolIndex, idol := range allIdols {

		// check if image has not been found and deleted, no need to loop through images if it has
		for i, img := range idol.BiasImages {
			if img.ObjectName == targetObjectName {

				// IMPORTANT: it is important that we do not delete the last image from the idol AND the idol from the all idols array. it MUST be one OR the other.

				// if that was the last image for the idol, delete idol from all idols
				if len(idol.BiasImages) == 1 {

					// remove pointer from array. struct will be garbage collected when not used by a game
					allIdols = append(allIdols[:idolIndex], allIdols[idolIndex+1:]...)
				} else {
					// delete image
					idol.BiasImages = append(idol.BiasImages[:i], idol.BiasImages[i+1:]...)
				}
				imageFound = true
				break IdolsLoop
			}
		}
	}
	allIdolsMutex.Unlock()
	// update idols
	setAllIdols(allIdols)

	// confirm an image was found and deleted
	if !imageFound {
		helpers.SendMessage(msg.ChannelID, "No image with that object name was found. No information was updated.")
		return
	}

	// create new image with given object name
	newIdolImage := IdolImage{
		ObjectName: targetObjectName,
	}

	// get image hash from object name
	img, _, err := helpers.DecodeImageBytes(newIdolImage.GetImgBytes())
	helpers.Relax(err)
	imgHash, err := helpers.GetImageHashString(img)
	helpers.Relax(err)
	newIdolImage.HashString = imgHash

	// attempt to get matching idol
	groupCheck, nameCheck, idolToUpdate := GetMatchingIdolAndGroup(newGroup, newName)

	// update database
	var idolsToUpdate []models.BiasGameIdolEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{"objectname": targetObjectName})).All(&idolsToUpdate)
	helpers.Relax(err)

	// if a database entry were found, update it
	if len(idolsToUpdate) == 1 {
		updateIdol := idolsToUpdate[0]
		updateIdol.Name = newName
		updateIdol.GroupName = newGroup
		updateIdol.Gender = newGender
		err := helpers.MDbUpsertID(models.BiasGameIdolsTable, updateIdol.ID, updateIdol)
		helpers.Relax(err)

		// if the new group/name already exists in memory, add image to that idol. otherwise create it
		if groupCheck && nameCheck && idolToUpdate != nil {
			allIdolsMutex.Lock()
			idolToUpdate.BiasImages = append(idolToUpdate.BiasImages, newIdolImage)
			allIdolsMutex.Unlock()
		} else {
			newIdol := makeIdolFromIdolEntry(updateIdol)
			setAllIdols(append(GetAllIdols(), &newIdol))
		}
	} else {
		// oh boy... these should not happen
		if len(idolsToUpdate) == 0 {
			helpers.SendMessage(msg.ChannelID, "No image with that object name was found IN MONGO, but the image was found memory. Data is out of sync, please refresh-images.")
		} else {
			helpers.SendMessage(msg.ChannelID, "To many images with that object name were found IN MONGO. This should never occur, please clean up the extra records manually and refresh-images")
		}
		return
	}

	// update cache
	if len(GetAllIdols()) > 0 {
		setModuleCache(ALL_IDOLS_CACHE_KEY, GetAllIdols(), time.Hour*24*7)
	}

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Image Update. Object Name: %s | Idol: %s %s", targetObjectName, newGroup, newName))

}

// deleteImage updates a specific image and its related idol info
func deleteImage(msg *discordgo.Message, content string) {
	contentArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	contentArgs = contentArgs[1:]

	// confirm amount of args
	if len(contentArgs) != 1 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	targetObjectName := contentArgs[0]

	allIdols := GetAllIdols()
	allIdolsMutex.Lock()
	imageFound := false

	// find and delete target image by object name
IdolLoop:
	for idolIndex, idol := range allIdols {

		// check if image has not been found and deleted, no need to loop through images if it has
		for i, bImg := range idol.BiasImages {
			if bImg.ObjectName == targetObjectName {

				// IMPORTANT: it is important that we do not delete the last image from the idol AND the idol from the all idols array. it MUST be one OR the other.

				// if that was the last image for the idol, delete idol from all idols
				if len(idol.BiasImages) == 1 {

					// if the whole idol is getting deleted, we need to load image
					//   bytes incase the image is being used by a game currently
					idol.BiasImages[i].ImageBytes = idol.BiasImages[i].GetImgBytes()

					// remove pointer from array. struct will be garbage collected when not used by a game
					allIdols = append(allIdols[:idolIndex], allIdols[idolIndex+1:]...)
				} else {
					// delete image
					idol.BiasImages = append(idol.BiasImages[:i], idol.BiasImages[i+1:]...)
				}
				imageFound = true
				break IdolLoop
			}
		}
	}
	allIdolsMutex.Unlock()
	// update idols
	setAllIdols(allIdols)

	// confirm an image was found and deleted
	if !imageFound {
		helpers.SendMessage(msg.ChannelID, "No image with that object name was found.")
		return
	}

	// update database
	var idolToDelete []models.BiasGameIdolEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{"objectname": targetObjectName})).All(&idolToDelete)
	helpers.Relax(err)

	// if a database entry were found, update it
	if len(idolToDelete) == 1 {

		// delete from database
		err := helpers.MDbDelete(models.BiasGameIdolsTable, idolToDelete[0].ID)
		helpers.Relax(err)

		// delete object
		helpers.DeleteFile(targetObjectName)
	}

	// update cache
	if len(GetAllIdols()) > 0 {
		setModuleCache(ALL_IDOLS_CACHE_KEY, GetAllIdols(), time.Hour*24*7)
	}

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Deleted image with object name: %s", targetObjectName))

}

// setAllIdols setter for all idols
func setAllIdols(idols []*Idol) {
	allIdolsMutex.Lock()
	defer allIdolsMutex.Unlock()

	allIdols = idols
}

// showImagesForIdol will show a embed message with all the available images for an idol
func showImagesForIdol(msg *discordgo.Message, msgContent string, showObjectNames bool) {
	defer helpers.Recover()
	cache.GetSession().ChannelTyping(msg.ChannelID)

	commandArgs, err := helpers.ToArgv(msgContent)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	commandArgs = commandArgs[1:]

	if len(commandArgs) < 2 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	// get matching idol to the group and name entered
	//  if we can't get one display an error
	groupMatch, nameMatch, matchIdol := GetMatchingIdolAndGroup(commandArgs[0], commandArgs[1])
	if matchIdol == nil || groupMatch == false || nameMatch == false {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-matching-idol"))
		return
	}

	// get bytes of all the images
	var idolImages []IdolImage
	for _, bImag := range matchIdol.BiasImages {
		idolImages = append(idolImages, bImag)
	}

	sendPagedEmbedOfImages(msg, idolImages, showObjectNames,
		fmt.Sprintf("Images for %s %s", matchIdol.GroupName, matchIdol.BiasName),
		fmt.Sprintf("Total Images: %s", humanize.Comma(int64(len(matchIdol.BiasImages)))))
}

// listIdolsInGame will list all idols that can show up in the biasgame
func listIdolsInGame(msg *discordgo.Message) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	genderCountMap := make(map[string]int)
	genderGroupCountMap := make(map[string]int)

	// create map of idols and there group
	groupIdolMap := make(map[string][]string)
	for _, bias := range GetAllIdols() {

		// count idols and groups
		genderCountMap[bias.Gender]++
		if _, ok := groupIdolMap[bias.GroupName]; !ok {
			genderGroupCountMap[bias.Gender]++
		}

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
			Name: "All Idols Available In Bias Game",
		},
		Title: fmt.Sprintf("%s Total | %s Girls, %s Boys | %s Girl Groups, %s Boy Groups",
			humanize.Comma(int64(len(GetAllIdols()))),
			humanize.Comma(int64(genderCountMap["girl"])),
			humanize.Comma(int64(genderCountMap["boy"])),
			humanize.Comma(int64(genderGroupCountMap["girl"])),
			humanize.Comma(int64(genderGroupCountMap["boy"])),
		),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Numbers show idols picture count",
		},
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
