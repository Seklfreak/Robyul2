package biasgame

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"github.com/nfnt/resize"

	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
)

// holds all available idols in the game
var allBiasChoices []*biasChoice
var allBiasesMutex sync.RWMutex

//////////////////////////////////
//     BIAS CHOICE FUNCTIONS    //
//////////////////////////////////

// will return a random image for the bias,
//  if an image has already been chosen for the given game and bias thenit will use that one
func (b *biasChoice) getRandomBiasImage(gameImageIndex *map[string]int) image.Image {
	var imageIndex int

	// check if a random image for the idol has already been chosen for this game
	//  also make sure that biasimages array contains the index. it may have been changed due to a refresh
	if imagePos, ok := (*gameImageIndex)[b.NameAndGroup]; ok && len(b.BiasImages) > imagePos {
		imageIndex = imagePos
	} else {
		imageIndex = rand.Intn(len(b.BiasImages))
		(*gameImageIndex)[b.NameAndGroup] = imageIndex
	}

	img, _, err := image.Decode(bytes.NewReader(b.BiasImages[imageIndex].getImgBytes()))
	helpers.Relax(err)
	return img
}

//////////////////////////////////
//     BIAS IMAGE FUNCTIONS     //
//////////////////////////////////

// will get the bytes to the correctly sized image bytes
func (b biasImage) getImgBytes() []byte {

	// image bytes is sometimes loaded if the object needs to be deleted
	if b.ImageBytes != nil {
		return b.ImageBytes
	}

	// get image bytes
	imgBytes, err := helpers.RetrieveFileWithoutLogging(b.ObjectName)
	helpers.Relax(err)

	img, _, err := helpers.DecodeImageBytes(imgBytes)
	helpers.Relax(err)

	// check if the image is already the correct size, otherwise resize it
	if img.Bounds().Dx() == IMAGE_RESIZE_HEIGHT && img.Bounds().Dy() == IMAGE_RESIZE_HEIGHT {
		return imgBytes
	} else {

		// resize image to the correct size
		img = resize.Resize(0, IMAGE_RESIZE_HEIGHT, img, resize.Lanczos3)

		// AFTER resizing, re-encode the bytes
		resizedImgBytes := new(bytes.Buffer)
		encoder := new(png.Encoder)
		encoder.CompressionLevel = -2
		encoder.Encode(resizedImgBytes, img)

		return resizedImgBytes.Bytes()
	}
}

// getMatchingIdolAndGroup will do a loose comparison of the name and group passed to the ones that already exist
//  1st return is true if group exists
//  2nd return is true if idol exists in the group
//  3rd will be a reference to the matching idol
func getMatchingIdolAndGroup(searchGroup, searchName string) (bool, bool, *biasChoice) {
	groupMatch := false
	nameMatch := false
	var matchingBiasChoice *biasChoice

	groupAliases := getGroupAliases()

	// create map of group => idols in group
	groupIdolMap := make(map[string][]*biasChoice)
	for _, bias := range getAllBiases() {
		groupIdolMap[bias.GroupName] = append(groupIdolMap[bias.GroupName], bias)
	}

	// check if the group suggested matches a current group. do loose comparison
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	for k, v := range groupIdolMap {
		curGroup := strings.ToLower(reg.ReplaceAllString(k, ""))
		sugGroup := strings.ToLower(reg.ReplaceAllString(searchGroup, ""))

		// check if group matches, if not check the aliases
		if curGroup == sugGroup {

			groupMatch = true
		} else {

			// if this group has any aliases check if the group we're
			//   searching for matches one of the aliases
		GroupLoop:
			for aliasGroup, aliases := range groupAliases {
				regGroup := strings.ToLower(reg.ReplaceAllString(aliasGroup, ""))
				if regGroup != curGroup {
					continue
				}

				for _, alias := range aliases {
					regAlias := strings.ToLower(reg.ReplaceAllString(alias, ""))
					if regAlias == sugGroup {
						groupMatch = true
						break GroupLoop
					}
				}
			}
		}

		if groupMatch {

			// check if the idols name matches
			for _, idol := range v {
				curName := strings.ToLower(reg.ReplaceAllString(idol.BiasName, ""))
				sugName := strings.ToLower(reg.ReplaceAllString(searchName, ""))

				if curName == sugName {
					nameMatch = true
					matchingBiasChoice = idol
					break
				}
			}
			break
		}
	}

	return groupMatch, nameMatch, matchingBiasChoice
}

// does a loose comparison of the group name to see if it exists
// return 1: if a matching group exists
// return 2: what the real group name is
func getMatchingGroup(searchGroup string) (bool, string) {

	allGroupsMap := make(map[string]bool)
	for _, bias := range getAllBiases() {
		allGroupsMap[bias.GroupName] = true
	}

	groupAliases := getGroupAliases()

	// check if the group suggested matches a current group. do loose comparison
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	for k, _ := range allGroupsMap {
		curGroup := strings.ToLower(reg.ReplaceAllString(k, ""))
		sugGroup := strings.ToLower(reg.ReplaceAllString(searchGroup, ""))

		// if groups match, set the suggested group to the current group
		if curGroup == sugGroup {
			return true, k
		}

		// if this group has any aliases check if the group we're
		//   searching for matches one of the aliases
		for aliasGroup, aliases := range groupAliases {
			regGroup := strings.ToLower(reg.ReplaceAllString(aliasGroup, ""))
			if regGroup != curGroup {
				continue
			}

			for _, alias := range aliases {
				regAlias := strings.ToLower(reg.ReplaceAllString(alias, ""))
				if regAlias == sugGroup {
					return true, k
				}
			}
		}
	}

	return false, ""
}

// refreshBiasChoices refreshes the list of bias choices.
//   initially called when bot starts but is also safe to call while bot is running if necessary
func refreshBiasChoices(skipCache bool) {

	if !skipCache {

		// attempt to get redis cache, return if its successful
		var tempAllBiases []*biasChoice
		err := getBiasGameCache("allbiaschoices", &tempAllBiases)
		if err == nil {
			setAllBiases(tempAllBiases)
			bgLog().Info("Biasgame images loaded from cache")
			return
		}

		bgLog().Info("Bias images loading from google drive. Cache not set or expired.")
	}

	var biasEntries []models.BiasGameIdolEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{})).All(&biasEntries)
	helpers.Relax(err)

	bgLog().Infof("Loading Bias Images. Images found: %d", len(biasEntries))

	var tempAllBiases []*biasChoice

	// run limited amount of goroutines at the same time
	mux := new(sync.Mutex)
	sem := make(chan bool, 50)
	for _, biasEntry := range biasEntries {
		sem <- true
		go func(biasEntry models.BiasGameIdolEntry) {
			defer func() { <-sem }()
			defer helpers.Recover()

			newBiasChoice := makeBiasChoiceFromBiasEntry(biasEntry)

			mux.Lock()
			defer mux.Unlock()

			// if the bias already exists, then just add this picture to the image array for the idol
			for _, currentBias := range tempAllBiases {
				if currentBias.NameAndGroup == newBiasChoice.NameAndGroup {
					currentBias.BiasImages = append(currentBias.BiasImages, newBiasChoice.BiasImages[0])
					return
				}
			}
			tempAllBiases = append(tempAllBiases, &newBiasChoice)
		}(biasEntry)
	}
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}

	bgLog().Info("Amount of idols loaded: ", len(tempAllBiases))
	setAllBiases(tempAllBiases)

	// cache all biases
	if len(getAllBiases()) > 0 {
		err = setBiasGameCache("allbiaschoices", getAllBiases(), time.Hour*24*7)
		helpers.RelaxLog(err)
	}
}

// makeBiasChoiceFromBiasEntry takes a mdb biasentry and makes a biasChoice to be used in the game
func makeBiasChoiceFromBiasEntry(entry models.BiasGameIdolEntry) biasChoice {
	bImage := biasImage{
		ObjectName: entry.ObjectName,
	}

	// get image hash string
	img, _, err := helpers.DecodeImageBytes(bImage.getImgBytes())
	helpers.Relax(err)
	imgHash, err := helpers.GetImageHashString(img)
	helpers.Relax(err)
	bImage.HashString = imgHash

	newBiasChoice := biasChoice{
		BiasName:     entry.Name,
		GroupName:    entry.GroupName,
		Gender:       entry.Gender,
		NameAndGroup: entry.Name + entry.GroupName,
		BiasImages:   []biasImage{bImage},
	}
	return newBiasChoice
}

// updateGroupStats if a target group is found, will update the group name
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
	if matched, realGroupName := getMatchingGroup(targetGroup); !matched {
		helpers.SendMessage(msg.ChannelID, "No group found with that exact name.")
		return
	} else {
		targetGroup = realGroupName
	}

	// update all idols in the target group
	var idolsUpdated int
	var allStatsUpdated int
	for _, idol := range getAllBiases() {
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
//  return 1: bias records update
//  return 2: stats records found
//  return 3: stats records update
func updateIdolInfo(targetGroup, targetName, newGroup, newName, newGender string) (int, int, int) {

	// attempt to find a matching idol of the new group and name,
	_, _, matchingBias := getMatchingIdolAndGroup(newGroup, newName)

	// update biases in memory
	recordsFound := 0
	statsFound := 0
	statsUpdated := 0
	allBiases := getAllBiases()
	allBiasesMutex.Lock()
	for biasIndex, targetBias := range allBiases {
		if targetBias.BiasName != targetName || targetBias.GroupName != targetGroup {
			continue
		}
		recordsFound++

		// if a matching idol was is found, just assign the targets images to it and delete
		if matchingBias != nil && (matchingBias.BiasName != targetBias.BiasName || matchingBias.GroupName != targetBias.GroupName) {

			matchingBias.BiasImages = append(matchingBias.BiasImages, targetBias.BiasImages...)
			allBiases = append(allBiases[:biasIndex], allBiases[biasIndex+1:]...)

			// update previous game stats
			statsFound, statsUpdated = updateGameStats(targetBias.GroupName, targetBias.BiasName, matchingBias.GroupName, matchingBias.BiasName, matchingBias.Gender)

		} else {

			// update previous game stats
			statsFound, statsUpdated = updateGameStats(targetBias.GroupName, targetBias.BiasName, newGroup, newName, newGender)

			// update targetbias name and group
			targetBias.BiasName = newName
			targetBias.GroupName = newGroup
			targetBias.Gender = newGender
		}
	}
	allBiasesMutex.Unlock()
	setAllBiases(allBiases)

	// update database
	var biasesToUpdate []models.BiasGameIdolEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{"groupname": targetGroup, "name": targetName})).All(&biasesToUpdate)
	helpers.Relax(err)

	for _, bias := range biasesToUpdate {
		bias.Name = newName
		bias.GroupName = newGroup
		bias.Gender = newGender

		err := helpers.MDbUpsertID(models.BiasGameIdolsTable, bias.ID, bias)
		helpers.Relax(err)
	}

	// update cache
	if len(getAllBiases()) > 0 {
		setBiasGameCache("allbiaschoices", getAllBiases(), time.Hour*24*7)
	}

	return recordsFound, statsFound, statsUpdated
}

// updateImageInfo updates a specific image and its related bias info
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

	allBiases := getAllBiases()
	allBiasesMutex.Lock()
	imageFound := false

	// find and delete target image by object name
BiasLoop:
	for biasIndex, bias := range allBiases {

		// check if image has not been found and deleted, no need to loop through images if it has
		for i, bImg := range bias.BiasImages {
			if bImg.ObjectName == targetObjectName {

				// IMPORTANT: it is important that we do not delete the last image from the bias AND the bias from the all biases array. it MUST be one OR the other.

				// if that was the last image for the idol, delete idol from all biases
				if len(bias.BiasImages) == 1 {

					// remove pointer from array. struct will be garbage collected when not used by a game
					allBiases = append(allBiases[:biasIndex], allBiases[biasIndex+1:]...)
				} else {
					// delete image
					bias.BiasImages = append(bias.BiasImages[:i], bias.BiasImages[i+1:]...)
				}
				imageFound = true
				break BiasLoop
			}
		}
	}
	allBiasesMutex.Unlock()
	// update biases
	setAllBiases(allBiases)

	// confirm an image was found and deleted
	if !imageFound {
		helpers.SendMessage(msg.ChannelID, "No image with that object name was found. No information was updated.")
		return
	}

	// create new image with given object name
	newBImg := biasImage{
		ObjectName: targetObjectName,
	}

	// get image hash from object name
	img, _, err := helpers.DecodeImageBytes(newBImg.getImgBytes())
	helpers.Relax(err)
	imgHash, err := helpers.GetImageHashString(img)
	helpers.Relax(err)
	newBImg.HashString = imgHash

	// attempt to get matching idol
	groupCheck, nameCheck, biasToUpdate := getMatchingIdolAndGroup(newGroup, newName)

	// update database
	var biasesToUpdate []models.BiasGameIdolEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{"objectname": targetObjectName})).All(&biasesToUpdate)
	helpers.Relax(err)

	// if a database entry were found, update it
	if len(biasesToUpdate) == 1 {
		updateBias := biasesToUpdate[0]
		updateBias.Name = newName
		updateBias.GroupName = newGroup
		updateBias.Gender = newGender
		err := helpers.MDbUpsertID(models.BiasGameIdolsTable, updateBias.ID, updateBias)
		helpers.Relax(err)

		// if the new group/name already exists in memory, add image to that bias. otherwise create it
		if groupCheck && nameCheck && biasToUpdate != nil {
			allBiasesMutex.Lock()
			biasToUpdate.BiasImages = append(biasToUpdate.BiasImages, newBImg)
			allBiasesMutex.Unlock()
		} else {
			newBiasChoice := makeBiasChoiceFromBiasEntry(updateBias)
			setAllBiases(append(getAllBiases(), &newBiasChoice))
		}
	} else {
		// oh boy... these should not happen
		if len(biasesToUpdate) == 0 {
			helpers.SendMessage(msg.ChannelID, "No image with that object name was found IN MONGO, but the image was found memory. Data is out of sync, please refresh-images.")
		} else {
			helpers.SendMessage(msg.ChannelID, "To many images with that object name were found IN MONGO. This should never occur, please clean up the extra records manually and refresh-images")
		}
		return
	}

	// update cache
	if len(getAllBiases()) > 0 {
		setBiasGameCache("allbiaschoices", getAllBiases(), time.Hour*24*7)
	}

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Image Update. Object Name: %s | Idol: %s %s", targetObjectName, newGroup, newName))

}

// updateImageInfo updates a specific image and its related bias info
func deleteBiasImage(msg *discordgo.Message, content string) {
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

	allBiases := getAllBiases()
	allBiasesMutex.Lock()
	imageFound := false

	// find and delete target image by object name
BiasLoop:
	for biasIndex, bias := range allBiases {

		// check if image has not been found and deleted, no need to loop through images if it has
		for i, bImg := range bias.BiasImages {
			if bImg.ObjectName == targetObjectName {

				// IMPORTANT: it is important that we do not delete the last image from the bias AND the bias from the all biases array. it MUST be one OR the other.

				// if that was the last image for the idol, delete idol from all biases
				if len(bias.BiasImages) == 1 {

					// if the whole bias is getting deleted, we need to load image
					//   bytes incase the image is being used by a game currently
					bias.BiasImages[i].ImageBytes = bias.BiasImages[i].getImgBytes()

					// remove pointer from array. struct will be garbage collected when not used by a game
					allBiases = append(allBiases[:biasIndex], allBiases[biasIndex+1:]...)
				} else {
					// delete image
					bias.BiasImages = append(bias.BiasImages[:i], bias.BiasImages[i+1:]...)
				}
				imageFound = true
				break BiasLoop
			}
		}
	}
	allBiasesMutex.Unlock()
	// update biases
	setAllBiases(allBiases)

	// confirm an image was found and deleted
	if !imageFound {
		helpers.SendMessage(msg.ChannelID, "No image with that object name was found.")
		return
	}

	// update database
	var biasToDelete []models.BiasGameIdolEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{"objectname": targetObjectName})).All(&biasToDelete)
	helpers.Relax(err)

	// if a database entry were found, update it
	if len(biasToDelete) == 1 {

		// delete from database
		err := helpers.MDbDelete(models.BiasGameIdolsTable, biasToDelete[0].ID)
		helpers.Relax(err)

		// delete object
		helpers.DeleteFile(targetObjectName)
	}

	// update cache
	if len(getAllBiases()) > 0 {
		setBiasGameCache("allbiaschoices", getAllBiases(), time.Hour*24*7)
	}

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Deleted image with object name: %s", targetObjectName))

}

// getAllBiases getter for all biases
func getAllBiases() []*biasChoice {
	allBiasesMutex.RLock()
	defer allBiasesMutex.RUnlock()

	if allBiasChoices == nil {
		return nil
	}

	return allBiasChoices
}

// setAllBiases setter for all biases
func setAllBiases(biases []*biasChoice) {
	allBiasesMutex.Lock()
	defer allBiasesMutex.Unlock()

	allBiasChoices = biases
}
