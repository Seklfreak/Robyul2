package idols

import (
	"bytes"
	"image/png"

	"github.com/Seklfreak/Robyul2/helpers"
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
