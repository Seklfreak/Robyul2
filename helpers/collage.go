package helpers

import (
	"C"
	"bytes"
	_ "image/png"
	"time"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/ungerik/go-cairo"
)
import (
	"image"
)

// Creates a Collage JPEG Image from the given imageUrls.
// imageUrls        : a slice with all image URLs. Empty strings will create an empty space in the collage.
// descriptions     : a slice with text that will be written on each tile. Can be empty.
// width            : the width of the result collage image.
// height           : the height of the result collage image.
// tileWidth        : the width of each tile image.
// tileHeight       : the height of each tile image.
// backgroundColour : the background colour as a hex string.
func CollageFromUrls(imageUrls, descriptions []string, width, height, tileWidth, tileHeight int, backgroundColourText string) (collageBytes []byte) {
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

	cairoSurface := cairo.NewSurface(cairo.FORMAT_RGB24, width, height)
	cairoSurface.SetSourceRGB(backgroundColour.R, backgroundColour.G, backgroundColour.B)
	cairoSurface.Paint()

	var posX, posY int

	for i, imageData := range imageDataArray {
		if posX > 0 && posX+tileWidth > width {
			posY += tileHeight
			posX = 0
		}
		if imageData != nil && len(imageData) > 0 {
			tileImage, _, err := image.Decode(bytes.NewReader(imageData))
			RelaxLog(err)
			tileSurface := cairo.NewSurfaceFromImage(tileImage)
			RelaxLog(err)
			if err == nil {
				cairoSurface.SetSourceSurface(tileSurface, float64(posX), float64(posY))
				cairoSurface.Paint()
			}
		}
		if len(descriptions) > i {
			// draw text
			cairoSurface.SetSourceRGB(0, 0, 0) // black
			cairoSurface.SelectFontFace("UnDotum", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
			cairoSurface.SetFontSize(28)
			cairoSurface.MoveTo(float64(posX+6), float64(posY+6+24))
			cairoSurface.ShowText(descriptions[i])
			// draw white outline to improve readability
			cairoSurface.MoveTo(float64(posX+6), float64(posY+6+24))
			cairoSurface.TextPath(descriptions[i])
			cairoSurface.SetSourceRGB(1, 1, 1) // white
			cairoSurface.SetLineWidth(4.5)
			cairoSurface.Stroke()
			// draw black outline to make text bold
			cairoSurface.MoveTo(float64(posX+6), float64(posY+6+24))
			cairoSurface.TextPath(descriptions[i])
			cairoSurface.SetSourceRGB(0, 0, 0) // black
			cairoSurface.SetLineWidth(2.0)
			cairoSurface.Stroke()
		}
		posX += tileWidth
	}

	bytesData, _ := cairoSurface.WriteToPNGStream()

	return bytesData
}
