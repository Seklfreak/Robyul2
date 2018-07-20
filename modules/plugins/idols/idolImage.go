package idols

import (
	"bytes"
	"fmt"
	"image/png"
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/nfnt/resize"
)

// GetImgBytes will get the bytes for the image with a default size of 150x150
func (i IdolImage) GetImgBytes() []byte {
	return i.GetResizeImgBytes(150)
}

// GetImageBytesWithResize will get the bytes to the correctly sized image bytes
func (i IdolImage) GetResizeImgBytes(resizeHeight int) []byte {

	// image bytes is sometimes loaded if the object needs to be deleted
	if i.ImageBytes != nil {
		return i.ImageBytes
	}

	// get image bytes
	imgBytes, err := helpers.RetrieveFileWithoutLogging(i.ObjectName)
	helpers.Relax(err)

	img, _, err := helpers.DecodeImageBytes(imgBytes)
	helpers.Relax(err)

	// check if the image is already the correct size, otherwise resize it
	if img.Bounds().Dx() == resizeHeight && img.Bounds().Dy() == resizeHeight {
		return imgBytes
	} else {

		// resize image to the correct size
		img = resize.Resize(0, uint(resizeHeight), img, resize.Lanczos3)

		// AFTER resizing, re-encode the bytes
		resizedImgBytes := new(bytes.Buffer)
		encoder := new(png.Encoder)
		encoder.CompressionLevel = -2
		encoder.Encode(resizedImgBytes, img)

		return resizedImgBytes.Bytes()
	}
}

// validateImages will read the idols table to retrieve all image object names. then it will make a call to retrieve all images
//  Note: to avoid spam, missing images object names are logged to console, not displayed in discord
func validateImages(msg *discordgo.Message, content string) {

	contentArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	// options
	listObjectName := false

	// check for options
	for _, option := range contentArgs {
		if option == "list" {
			listObjectName = true
		}
	}

	helpers.SendMessage(msg.ChannelID, "Checking idol images..")

	// loop through idol images and check if object exists
	var missingImages []string
	for _, idol := range GetActiveIdols() {
		for _, image := range idol.Images {
			if !helpers.ObjectExists(image.ObjectName) {
				missingImages = append(missingImages, image.ObjectName)
				log().Infoln("Idol image does not exist in minio: ", image.ObjectName)
			}
		}
	}

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Done.\nMissing Images: %d", len(missingImages)))

	// list out object names if wanted
	if listObjectName {
		printableObjectNames := strings.Join(missingImages, "\n")
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Missing Image Object Names: \n%s", printableObjectNames))
	}
}
