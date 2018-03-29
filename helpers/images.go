package helpers

import (
	"bytes"
	"image"
	"image/draw"
	"io"

	"github.com/corona10/goimagehash"
)

// CombineTwoImages combines two images with img1 being on the left and img2 on the right. returns the resulting image
func CombineTwoImages(img1 image.Image, img2 image.Image) image.Image {

	//starting position of the second image (bottom left)
	sp2 := image.Point{img1.Bounds().Dx(), 0}

	//new rectangle for the second image
	r2 := image.Rectangle{sp2, sp2.Add(img2.Bounds().Size())}

	//rectangle for the big image
	r := image.Rectangle{image.Point{0, 0}, r2.Max}
	rgba := image.NewRGBA(r)

	draw.Draw(rgba, img1.Bounds(), img1, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, r2, img2, image.Point{0, 0}, draw.Src)

	return rgba.SubImage(r)
}

// decodeImage decodes the image with retry.
func DecodeImage(r io.ReadCloser) (image.Image, error) {

	// decode image
	img, _, imgErr := image.Decode(r)
	if imgErr != nil {

		// if image fails decoding, which has been happening randomly. attempt to decode it again up to 5 times
		for i := 0; i < 5; i++ {
			img, _, imgErr = image.Decode(r)

			if imgErr == nil {
				break
			}
		}

		// if image still can't be decoded, then leave it out of the game
		if imgErr != nil {
			return nil, imgErr
		}
	}

	return img, nil
}

// ImageHashComparison does a comparison of the given hashes and returns value indicating their difference
//   0 is perfect match
func ImageHashStringComparison(imgHashString1, imgHashString2 string) (int, error) {
	imgHash1, err := goimagehash.ImageHashFromString(imgHashString1)
	if err != nil {
		return -1, err
	}
	imgHash2, err := goimagehash.ImageHashFromString(imgHashString2)
	if err != nil {
		return -1, err
	}

	return imgHash1.Distance(imgHash2)
}

// ImageHashComparison does an image comparison using the average hash
//   0 is perfect match
func ImageComparison(img1, img2 image.Image) (int, error) {
	hash1, err := GetImageHash(img1)
	if err != nil {
		return -1, err
	}

	hash2, err := GetImageHash(img2)
	if err != nil {
		return -1, err
	}

	return hash1.Distance(hash2)
}

// ImageHashComparison does an image comparison using the average hash
//   0 is perfect match
func ImageByteComparison(imgByte1, imgByte2 []byte) (int, error) {
	img1, _, err := DecodeImageBytes(imgByte1)
	if err != nil {
		return -1, err
	}
	img2, _, err := DecodeImageBytes(imgByte2)
	if err != nil {
		return -1, err
	}

	return ImageComparison(img1, img2)
}

// GetImageHash returns the average image hash
//  note: use helpers.GetImageHash(img).ToString() to get the string value
func GetImageHash(img image.Image) (*goimagehash.ImageHash, error) {
	return goimagehash.AverageHash(img)
}

// GetImageHash returns the average image hash
//  note: use helpers.GetImageHash(img).ToString() to get the string value
func GetImageHashString(img image.Image) (string, error) {
	sugImgHash, err := GetImageHash(img)
	if err != nil {
		return "", err
	}
	return sugImgHash.ToString(), nil
}

// DecodeImageBytes
func DecodeImageBytes(imageBytes []byte) (image.Image, string, error) {
	return image.Decode(bytes.NewReader(imageBytes))
}
