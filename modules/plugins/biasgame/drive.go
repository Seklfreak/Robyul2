package biasgame

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mgutz/str"

	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/nfnt/resize"
	"github.com/sethgrid/pester"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// startBiasCacheRefreshLoop will refresh the image cache for both misc image and bias images
func startBiasCacheRefreshLoop() {
	bgLog().Info("Starting biasgame refresh image cache loop")

	go func() {
		defer helpers.Recover()

		for {
			// refresh every 12 hours
			time.Sleep(time.Hour * 12)

			bgLog().Info("Refreshing image cache...")
			refreshBiasChoices(true)

			bgLog().Info("Biasgame image cache has been refresh")
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

	if skipCache == false {

		// attempt to get redis cache, return if its successful
		var tempAllBiases []*biasChoice
		err := getBiasGameCache("allbiaschoices", &tempAllBiases)
		if err == nil {
			setAllBiases(tempAllBiases)
			bgLog().Info("Biasgame images loaded from cache")
			return
		}

		bgLog().Info("Bias iamges loading from google drive. Cache not set or expired.")
	}

	var biasEntries []models.BiasEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{})).All(&biasEntries)
	helpers.Relax(err)

	bgLog().Infof("Loading Bias Images. Images found: %d", len(biasEntries))

	var tempAllBiases []*biasChoice

	var wg sync.WaitGroup
	mux := new(sync.Mutex)
	for _, biasEntry := range biasEntries {
		wg.Add(1)
		go func(biasEntry models.BiasEntry) {
			defer wg.Done()

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
	wg.Wait()

	bgLog().Info("Amount of idols loaded: ", len(tempAllBiases))
	setAllBiases(tempAllBiases)

	// cache all biases
	if len(getAllBiases()) > 0 {
		setBiasGameCache("allbiaschoices", getAllBiases(), time.Hour*24*7)
	}
}

// addDriveFileToAllBiases will take a drive file, convert it to a bias object,
//   and add it to allBiasChoices or add a new image if the idol already exists
func addSuggestionToGame(suggestion *models.BiasGameSuggestionEntry) {

	// get suggestion details and add to biasEntry table
	biasEntry := models.BiasEntry{
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
func makeBiasChoiceFromBiasEntry(entry models.BiasEntry) biasChoice {
	// get image bytes
	imgBytes, err := helpers.RetrieveFile(entry.ObjectName)
	helpers.Relax(err)

	// resize image to the correct size
	img, _, err := helpers.DecodeImageBytes(imgBytes)
	img = resize.Resize(0, IMAGE_RESIZE_HEIGHT, img, resize.Lanczos3)

	// AFTER resizing, re-encode the bytes
	resizedImgBytes := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = -2
	encoder.Encode(resizedImgBytes, img)

	// get image hash string
	helpers.Relax(err)
	imgHash, err := helpers.GetImageHashString(img)
	helpers.Relax(err)

	bImage := biasImage{
		ImageBytes: resizedImgBytes.Bytes(),
		HashString: imgHash,
		ObjectName: entry.ObjectName,
	}

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
	contentArgs := str.ToArgv(content)[1:]

	// confirm amount of args
	if len(contentArgs) < 4 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	targetGroup := contentArgs[0]
	targetName := contentArgs[1]
	newGroup := contentArgs[2]
	newName := contentArgs[3]

	// if a gender was passed, make sure its valid
	if len(contentArgs) == 5 {
		if contentArgs[4] != "boy" && contentArgs[4] != "girl" {
			helpers.SendMessage(msg.ChannelID, "Invalid gender. Gender must be exactly 'girl' or 'boy'. No information was updated.")
			return
		}
	}

	// update biases in memory
	recordsFound := 0
	allBiases := getAllBiases()
	allBiasesMutex.Lock()
	for _, bias := range allBiases {
		if bias.BiasName != targetName || bias.GroupName != targetGroup {
			continue
		}
		recordsFound++

		bias.BiasName = newName
		bias.GroupName = newGroup
		if len(contentArgs) == 5 && (contentArgs[4] == "boy" || contentArgs[4] == "girl") {
			bias.Gender = contentArgs[4]
		}
	}
	allBiasesMutex.Unlock()

	// check if nothing was found
	if recordsFound == 0 {
		helpers.SendMessage(msg.ChannelID, "No Idols found with that exact group and name.")
		return
	}

	// update database
	var biasesToUpdate []models.BiasEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{"groupname": targetGroup, "name": targetName})).All(&biasesToUpdate)
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

	// get files from drive
	girlFiles := getFilesFromDriveFolder(GIRLS_FOLDER_ID)
	boyFiles := getFilesFromDriveFolder(BOYS_FOLDER_ID)
	allFiles := append(girlFiles, boyFiles...)

	amountMigrated := 0

	// confirm files were found
	if len(allFiles) > 0 {

		bgLog().Info("--Migrating google drive biasgame images to object storage. Total images found: ", len(allFiles))
		for _, file := range allFiles {
			// determine gender from folder
			var gender string
			if file.Parents[0] == GIRLS_FOLDER_ID {
				gender = "girl"
			} else {
				gender = "boy"
			}

			// get bias name and group name from file name
			groupBias := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))

			biasEntry := models.BiasEntry{
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
		fmt.Printf("Error getting google drive files from folderid: %s\n%s\n", folderId, err.Error())
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
