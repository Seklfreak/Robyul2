package biasgame

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/nfnt/resize"
	"github.com/sethgrid/pester"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// loadMiscImages handles loading other images besides the idol images
func loadMiscImages() {

	miscFiles := getFilesFromDriveFolder(MISC_FOLDER_ID)
	bgLog().Info("Loading biasgame misc images...")

	for _, file := range miscFiles {
		res, err := http.Get(file.WebContentLink)
		if err != nil {
			return
		}
		img, _, err := image.Decode(res.Body)
		if err != nil {
			continue
		}

		switch file.Name {
		case "verses16.png":
			bgLog().Info("Loaded verses image")

			// resize verses image to match the bias image sizes
			resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
			versesImage = resizedImage

		case "topEightBracket.png":

			bgLog().Info("Loaded top eight bracket image")
			winnerBracket = img
		case "shadow-border.png":

			bgLog().Info("Loaded shadow border image")
			resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
			shadowBorder = resizedImage
		case "crown.png":

			bgLog().Info("Loaded crown image")
			resizedImage := resize.Resize(IMAGE_RESIZE_HEIGHT/2, 0, img, resize.Lanczos3)
			crown = resizedImage
		}
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

	// check if redis cache exists, if so load from cache if not explicitly skipping cache
	cacheResult, err := cache.GetRedisClient().Get("testbiasgamecache-all").Bytes()
	if err == nil && err != redis.Nil && skipCache == false {
		bgLog().Info("Biasgame images loaded from cache")
		json.Unmarshal(cacheResult, &allBiasChoices)
		return
	}

	// get idol image from google drive
	girlFiles := getFilesFromDriveFolder(GIRLS_FOLDER_ID)
	boyFiles := getFilesFromDriveFolder(BOYS_FOLDER_ID)
	allFiles := append(girlFiles, boyFiles...)

	if len(allFiles) > 0 {
		var wg sync.WaitGroup
		mux := new(sync.Mutex)

		// set up temp array and load that first to avoid issues with a user startin a game while the biases are being refreshed
		var tempAllBiases []*biasChoice

		bgLog().Info("Loading Biasgame Images. Total images found: ", len(allFiles))
		for i, file := range allFiles {
			wg.Add(1)

			go func(index int, file *drive.File) {
				defer wg.Done()

				newBiasChoice, err := makeBiasChoiceFromDriveFile(file)
				if err != nil {
					return
				}
				bgLog().Infof("Loading bias: Name: %s, Group: %s, File: %s", newBiasChoice.BiasName, newBiasChoice.GroupName, newBiasChoice.FileName)

				mux.Lock()
				defer mux.Unlock()

				// if the bias already exists, then just add this picture to the image array for the idol
				for _, currentBias := range tempAllBiases {
					if currentBias.FileName == newBiasChoice.FileName {
						currentBias.BiasImages = append(currentBias.BiasImages, newBiasChoice.BiasImages[0])
						return
					}
				}

				tempAllBiases = append(tempAllBiases, &newBiasChoice)
			}(i, file)
		}
		wg.Wait()

		bgLog().Info("Amount of idols loaded: ", len(tempAllBiases))
		allBiasChoices = tempAllBiases

		// cache all biases
		marshaledBiasChoices, err := json.Marshal(tempAllBiases)
		helpers.Relax(err)
		cache.GetRedisClient().Set("testbiasgamecache-all", marshaledBiasChoices, time.Minute*15)

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

// makeBiasChoiceFromDriveFile
func makeBiasChoiceFromDriveFile(file *drive.File) (biasChoice, error) {
	res, err := pester.Get(file.WebContentLink)
	if err != nil {
		fmt.Println("get error: ", err.Error())
		return biasChoice{}, err
	}

	// decode image
	img, imgErr := helpers.DecodeImage(res.Body)
	if imgErr != nil {
		fmt.Printf("error decoding image %s:\n %s", file.Name, imgErr)
		return biasChoice{}, imgErr
	}

	resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT, img, resize.Lanczos3)

	// get bias name and group name from file name
	groupBias := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))

	var gender string
	if file.Parents[0] == GIRLS_FOLDER_ID {
		gender = "girl"
	} else {
		gender = "boy"
	}

	var imageBuffer bytes.Buffer
	err = png.Encode(&imageBuffer, resizedImage)
	helpers.Relax(err)
	imageBytes := imageBuffer.Bytes()

	newBiasChoice := biasChoice{
		FileName:       file.Name,
		DriveId:        file.Id,
		WebViewLink:    file.WebViewLink,
		WebContentLink: file.WebContentLink,
		GroupName:      strings.Split(groupBias, "_")[0],
		BiasName:       strings.Split(groupBias, "_")[1],
		BiasImages:     [][]byte{imageBytes},
		Gender:         gender,
	}

	return newBiasChoice, nil
}

// addDriveFileToAllBiases will take a drive file, convert it to a bias object,
//   and add it to allBiasChoices or add a new image if the idol already exists
func addDriveFileToAllBiases(file *drive.File) {
	newBiasChoice, err := makeBiasChoiceFromDriveFile(file)
	if err != nil {
		return
	}

	// if the bias already exists, then just add this picture to the image array for the idol
	for _, currentBias := range allBiasChoices {
		if currentBias.FileName == newBiasChoice.FileName {
			currentBias.BiasImages = append(currentBias.BiasImages, newBiasChoice.BiasImages[0])
			return
		}
	}

	allBiasChoices = append(allBiasChoices, &newBiasChoice)
}
