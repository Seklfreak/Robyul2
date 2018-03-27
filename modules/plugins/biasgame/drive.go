package biasgame

import (
	"bytes"
	"fmt"
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

// startBiasCacheRefreshLoop will refresh the image cache for both misc image and bias images
func startBiasCacheRefreshLoop() {
	bgLog().Info("Starting biasgame refresh image cache loop")

	go func() {
		defer helpers.Recover()

		for {
			// refresh every 12 hours
			time.Sleep(time.Hour * 12)

			bgLog().Info("Refreshing image cache...")
			loadMiscImages(true)
			refreshBiasChoices(true)

			bgLog().Info("Biasgame image cache has been refresh")
		}
	}()
}

// loadMiscImages handles loading other images besides the idol images
func loadMiscImages(skipCache bool) {
	// skipCache = true
	bgLog().Info("Loading biasgame misc images")
	validMiscImages := map[string]bool{
		"verses16.png":        true,
		"topEightBracket.png": true,
		"shadow-border.png":   true,
		"crown.png":           true,
	}

	// loop through all the files in the misc folder
	for _, file := range getFilesFromDriveFolder(MISC_FOLDER_ID) {

		// make sure other files in the misc folder aren't loaded
		if validMiscImages[file.Name] == false {
			continue
		}

		var img image.Image
		var err error

		// check if image is cached
		var imgBuf []byte
		err = getBiasGameCache(file.Name, &imgBuf)
		if err == nil && skipCache == false {

			// decode image and set the appropriate var
			img, _, err = image.Decode(bytes.NewReader(imgBuf))
			if err == nil {
				switch file.Name {
				case "verses16.png":
					versesImage = img
				case "topEightBracket.png":
					winnerBracket = img
				case "shadow-border.png":
					shadowBorder = img
				case "crown.png":
					crown = img
				}
				bgLog().Infof("Biasgame misc image loaded from cache: %s", file.Name)
				continue
			}
		}

		// get image and decode it
		res, err := http.Get(file.WebContentLink)
		if err != nil {
			bgLog().Errorf("Error loading misc image '%s'!! Error: %s", err.Error())
			return
		}
		img, _, err = image.Decode(res.Body)
		if err != nil {
			bgLog().Errorf("Error decoding misc image '%s'!! Error: %s", err.Error())
			continue
		}

		bgLog().Infof("Loading biasgame misc image: %s", file.Name)
		switch file.Name {
		case "verses16.png":

			// resize verses image to match the bias image sizes
			img = resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
			versesImage = img
		case "topEightBracket.png":

			winnerBracket = img
		case "shadow-border.png":

			img = resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
			shadowBorder = img
		case "crown.png":

			img = resize.Resize(IMAGE_RESIZE_HEIGHT/2, 0, img, resize.Lanczos3)
			crown = img
		}

		// cache misc image
		buf := new(bytes.Buffer)
		err = png.Encode(buf, img)
		if err == nil {
			bgLog().Infof("Setting cache for: %s", file.Name)
			setBiasGameCache(file.Name, buf.Bytes(), time.Hour*24*7)
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
				// bgLog().Infof("Loading bias: Name: %s, Group: %s, File: %s", newBiasChoice.BiasName, newBiasChoice.GroupName, newBiasChoice.FileName)

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
		setAllBiases(tempAllBiases)

		// cache all biases
		if len(getAllBiases()) > 0 {
			setBiasGameCache("allbiaschoices", getAllBiases(), time.Hour*24*7)
		}

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

	// encode image with png encoding
	var imageBuffer bytes.Buffer
	err = png.Encode(&imageBuffer, resizedImage)
	helpers.Relax(err)
	imgBytes := imageBuffer.Bytes()

	// create the first biasImage
	imgHash, err := helpers.GetImageHashString(resizedImage)
	helpers.Relax(err)
	bImg := biasImage{
		ImageBytes: imgBytes,
		HashString: imgHash,
	}

	newBiasChoice := biasChoice{
		FileName:       file.Name,
		DriveId:        file.Id,
		WebViewLink:    file.WebViewLink,
		WebContentLink: file.WebContentLink,
		GroupName:      strings.Split(groupBias, "_")[0],
		BiasName:       strings.Split(groupBias, "_")[1],
		BiasImages:     []biasImage{bImg},
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
	for _, currentBias := range getAllBiases() {
		if currentBias.FileName == newBiasChoice.FileName {
			currentBias.BiasImages = append(currentBias.BiasImages, newBiasChoice.BiasImages[0])
			return
		}
	}

	setAllBiases(append(getAllBiases(), &newBiasChoice))

	// cache all biases
	if len(getAllBiases()) > 0 {
		setBiasGameCache("allbiaschoices", getAllBiases(), time.Hour*24*7)
	}
}
