package helpers

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"bytes"

	"image/png"

	"github.com/nfnt/resize"
)

// Scales an image to the target size. Source has to be a JPEG, PNG or GIF. Result will be a PNG.
// data			: the image to scale
// targetWidth	: the target width
// targetHeight	: the target height
func ScaleImage(data []byte, targetWidth, targetHeight int) (result []byte, err error) {
	// decode image
	sourceImage, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// resizes the image
	targetImage := resize.Resize(uint(targetWidth), uint(targetHeight), sourceImage, resize.Bilinear)

	// encode it to a png
	var buff bytes.Buffer
	err = png.Encode(&buff, targetImage)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}
