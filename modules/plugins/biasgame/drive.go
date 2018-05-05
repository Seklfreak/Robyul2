package biasgame

import (
	"fmt"
	"image"
	"image/draw"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/nfnt/resize"
	"github.com/sethgrid/pester"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// startCacheRefreshLoop will refresh the image cache for both misc image and bias images
func startCacheRefreshLoop() {
	bgLog().Info("Starting biasgame refresh image cache loop")

	go func() {
		defer helpers.Recover()

		for {
			time.Sleep(time.Hour * 12)

			bgLog().Info("Refreshing image cache...")
			refreshBiasChoices(true)

			bgLog().Info("Biasgame image cache has been refresh")
		}
	}()

	bgLog().Info("Starting biasgame current games cache loop")
	go func() {
		defer helpers.Recover()

		for {
			time.Sleep(time.Second * 30)

			// save any currently running games
			err := setBiasGameCache("currentSinglePlayerGames", getCurrentSinglePlayerGames(), 0)
			helpers.Relax(err)
			bgLog().Infof("Cached %d singleplayer biasgames to redis", len(getCurrentSinglePlayerGames()))

			err = setBiasGameCache("currentMultiPlayerGames", getCurrentMultiPlayerGames(), 0)
			helpers.Relax(err)
			bgLog().Infof("Cached %d multiplayer biasgames to redis", len(getCurrentMultiPlayerGames()))
		}
	}()

}

// loadMiscImages handles loading other images besides the idol images
func loadMiscImages() {
	validMiscImages := []string{
		"verses.png",
		"top-eight-bracket.png",
		"shadow-border.png",
		"crown.png",
	}

	miscImagesFolderPath := helpers.GetConfig().Path("assets_folder").Data().(string) + "biasgame/misc/"

	// load misc images
	for _, fileName := range validMiscImages {

		// check if file exists
		filePath := miscImagesFolderPath + fileName
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			helpers.Relax(err)
		}

		// open file and decode it
		file, err := os.Open(filePath)
		helpers.Relax(err)
		img, _, err := image.Decode(file)
		helpers.Relax(err)

		// resize misc images as needed
		switch fileName {
		case "verses.png":
			versesImage = resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
		case "shadow-border.png":
			shadowBorder = resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
		case "crown.png":
			crown = resize.Resize(IMAGE_RESIZE_HEIGHT/2, 0, img, resize.Lanczos3)
		case "top-eight-bracket.png":
			winnerBracket = img
		}
		bgLog().Infof("Loading biasgame misc image: %s", fileName)
	}

	// append crown to top eight
	bracketImage := image.NewRGBA(winnerBracket.Bounds())
	draw.Draw(bracketImage, winnerBracket.Bounds(), winnerBracket, image.Point{0, 0}, draw.Src)
	draw.Draw(bracketImage, crown.Bounds().Add(image.Pt(230, 5)), crown, image.ZP, draw.Over)
	winnerBracket = bracketImage.SubImage(bracketImage.Rect)
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

// addDriveFileToAllBiases will take a drive file, convert it to a bias object,
//   and add it to allBiasChoices or add a new image if the idol already exists
func addSuggestionToGame(suggestion *models.BiasGameSuggestionEntry) {

	// get suggestion details and add to biasEntry table
	biasEntry := models.BiasGameIdolEntry{
		ID:         "",
		Gender:     suggestion.Gender,
		GroupName:  suggestion.GrouopName,
		Name:       suggestion.Name,
		ObjectName: suggestion.ObjectName,
	}

	// insert file to mongodb
	_, err := helpers.MDbInsert(models.BiasGameIdolsTable, biasEntry)
	helpers.Relax(err)

	newBiasChoice := makeBiasChoiceFromBiasEntry(biasEntry)

	// if the bias already exists, then just add this picture to the image array for the idol
	biasExists := false
	for _, currentBias := range getAllBiases() {
		if currentBias.NameAndGroup == newBiasChoice.NameAndGroup {
			currentBias.BiasImages = append(currentBias.BiasImages, newBiasChoice.BiasImages[0])
			biasExists = true
			break
		}
	}

	// if its a new bias, update all biases array
	if biasExists == false {
		setAllBiases(append(getAllBiases(), &newBiasChoice))
	}

	// cache all biases
	if len(getAllBiases()) > 0 {
		setBiasGameCache("allbiaschoices", getAllBiases(), time.Hour*24*7)
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

// updateIdolInfo updates a idols group, name, and/or gender depending on args
func updateIdolInfo(msg *discordgo.Message, content string) {

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

	targetGroup := contentArgs[0]
	targetName := contentArgs[1]
	newGroup := contentArgs[2]
	newName := contentArgs[3]
	updateGender := false

	// if a gender was passed, make sure its valid
	if len(contentArgs) == 5 {
		updateGender = true
		if contentArgs[4] != "boy" && contentArgs[4] != "girl" {
			helpers.SendMessage(msg.ChannelID, "Invalid gender. Gender must be exactly 'girl' or 'boy'. No information was updated.")
			return
		}
	}

	// attempt to find a matching idol of the new group and name,
	_, _, matchingBias := getMatchingIdolAndGroup(newGroup, newName)

	// update biases in memory
	recordsFound := 0
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
			updateGameStats(targetBias.GroupName, targetBias.BiasName, matchingBias.GroupName, matchingBias.BiasName, matchingBias.Gender)

		} else {

			// update previous game stats
			updateGameStats(targetBias.GroupName, targetBias.BiasName, newGroup, newName, contentArgs[4])

			// update targetbias name and group
			targetBias.BiasName = newName
			targetBias.GroupName = newGroup
			if updateGender {
				targetBias.Gender = contentArgs[4]
			}

		}
	}
	allBiasesMutex.Unlock()
	setAllBiases(allBiases)

	// check if nothing was found
	if recordsFound == 0 {
		helpers.SendMessage(msg.ChannelID, "No Idols found with that exact group and name.")
		return
	}

	// update database
	var biasesToUpdate []models.BiasGameIdolEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{"groupname": targetGroup, "name": targetName})).All(&biasesToUpdate)
	helpers.Relax(err)

	for _, bias := range biasesToUpdate {
		bias.Name = newName
		bias.GroupName = newGroup

		if len(contentArgs) == 5 && (contentArgs[4] == "boy" || contentArgs[4] == "girl") {
			bias.Gender = contentArgs[4]
		}

		err := helpers.MDbUpsertID(models.BiasGameIdolsTable, bias.ID, bias)
		helpers.Relax(err)
	}

	// update cache
	if len(getAllBiases()) > 0 {
		setBiasGameCache("allbiaschoices", getAllBiases(), time.Hour*24*7)
	}

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Idol Information updated. Old: %s %s | New: %s %s", targetGroup, targetName, newGroup, newName))
}

// runGoogleDriveMigration Should only be run on rare occasions when issues occur with object storage or setting up a new object storage
//  note: takes a very long time to complete
func runGoogleDriveMigration(msg *discordgo.Message) {
	girlFolderId := helpers.GetConfig().Path("biasgame.girl_folder_id").Data().(string)
	boyFolderId := helpers.GetConfig().Path("biasgame.boy_folder_id").Data().(string)

	// get files from drive
	girlFiles := getFilesFromDriveFolder(girlFolderId)
	boyFiles := getFilesFromDriveFolder(boyFolderId)
	allFiles := append(girlFiles, boyFiles...)

	amountMigrated := 0

	// confirm files were found
	if len(allFiles) > 0 {

		bgLog().Info("--Migrating google drive biasgame images to object storage. Total images found: ", len(allFiles))
		for _, file := range allFiles {
			// determine gender from folder
			var gender string
			if file.Parents[0] == girlFolderId {
				gender = "girl"
			} else {
				gender = "boy"
			}

			// get bias name and group name from file name
			groupBias := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))

			biasEntry := models.BiasGameIdolEntry{
				ID:        "",
				DriveID:   file.Id,
				Gender:    gender,
				GroupName: strings.Split(groupBias, "_")[0],
				Name:      strings.Split(groupBias, "_")[1],
			}

			// check if a record with this drive id already exists
			//  this means its been migrated before and should not be remigrated
			count, err := helpers.MdbCount(models.BiasGameIdolsTable, bson.M{"driveid": biasEntry.DriveID})
			if err != nil {
				bgLog().Errorf("Error getting count for drive id '%s'. Error: %s", biasEntry.DriveID, err.Error())
				continue
			}
			if count != 0 {
				bgLog().Infof("Drive id '%s' has already been migrated. Skipping", biasEntry.DriveID)
				continue
			}
			bgLog().Infof("Migrating Drive id '%s'. Idol Name: %s | Group Name: %s", biasEntry.DriveID, biasEntry.Name, biasEntry.GroupName)

			// get image
			res, err := pester.Get(file.WebContentLink)
			helpers.Relax(err)
			imgBytes, err := ioutil.ReadAll(res.Body)

			// store file in object storage
			objectName, err := helpers.AddFile("", imgBytes, helpers.AddFileMetadata{
				Filename:           file.WebContentLink,
				ChannelID:          msg.ChannelID,
				UserID:             msg.Author.ID,
				AdditionalMetadata: nil,
			}, "biasgame", false)

			// set object name
			biasEntry.ObjectName = objectName

			// insert file to mongodb
			_, err = helpers.MDbInsert(models.BiasGameIdolsTable, biasEntry)
			if err != nil {
				bgLog().Errorf("Error migrating drive id '%s'. Error: %s", biasEntry.DriveID, err.Error())
			}
			amountMigrated++
		}
		bgLog().Info("--Google drive migration complete--")
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Migration Complete. Files Migrated: %d", amountMigrated))

	} else {
		bgLog().Warn("No biasgame file found!")
	}
}

// getFilesFromDriveFolder
func getFilesFromDriveFolder(folderId string) []*drive.File {
	driveService := cache.GetGoogleDriveService()

	// get girls image from google drive
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, folderId)).Fields(googleapi.Field("nextPageToken, files(name, id, parents, webViewLink, webContentLink)")).PageSize(1000).Do()
	if err != nil {
		return nil
	}
	allFiles := results.Files

	// retry for more bias images if needed
	pageToken := results.NextPageToken
	for pageToken != "" {
		results, err = driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, folderId)).Fields(googleapi.Field("nextPageToken, files(name, id, parents, webViewLink, webContentLink)")).PageSize(1000).PageToken(pageToken).Do()
		pageToken = results.NextPageToken
		if len(results.Files) > 0 {
			allFiles = append(allFiles, results.Files...)
		} else {
			break
		}
	}

	return allFiles
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
