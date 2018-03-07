package helpers

import (
	"bufio"
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"time"

	"github.com/lucasb-eyer/go-colorful"
)

// Creates a Collage JPEG Image from the given imageUrls.
// imageUrls        : a slice with all image URLs. Empty strings will create an empty space in the collage.
// width            : the width of the result collage image.
// height           : the height of the result collage image.
// tileWidth        : the width of each tile image.
// tileHeight       : the height of each tile image.
// backgroundColour : the background colour as a hex string.
func CollageFromUrls(imageUrls []string, width, height, tileWidth, tileHeight int, backgroundColourText string) (collageBytes []byte) {
	imageDataArray := make([][]byte, 0)
	for _, imageUrl := range imageUrls {
		if imageUrl == "" {
			imageDataArray = append(imageDataArray, nil)
			continue
		}
		imageData, err := NetGetUAWithErrorAndTimeout(imageUrl, DEFAULT_UA, 15*time.Second)
		RelaxLog(err)
		if err == nil {
			imageDataArray = append(imageDataArray, imageData)
		} else {
			imageDataArray = append(imageDataArray, nil)
		}
	}

	backgroundColour, _ := colorful.Hex(backgroundColourText)

	collageImage := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(collageImage, collageImage.Bounds(), image.NewUniform(backgroundColour), image.ZP, draw.Src)

	var posX, posY int

	for _, imageData := range imageDataArray {
		if posX > 0 && posX+tileWidth > width {
			posY += tileHeight
			posX = 0
		}
		if imageData != nil && len(imageData) > 0 {
			tileImage, _, err := image.Decode(bytes.NewReader(imageData))
			RelaxLog(err)
			if err == nil {
				draw.Draw(
					collageImage,
					tileImage.Bounds().Add(image.Pt(posX, posY)),
					tileImage,
					image.Point{0, 0},
					draw.Src,
				)
			}
		}
		posX += tileWidth
	}

	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)

	jpeg.Encode(writer, collageImage, &jpeg.Options{95})

	return buffer.Bytes()
}
