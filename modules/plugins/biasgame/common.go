package biasgame

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"math/rand"
	"time"

	"github.com/Seklfreak/Robyul2/modules/plugins/idols"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis"
	json "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
)

// giveImageShadowBorder give the round image a shadow border
func giveImageShadowBorder(img image.Image, offsetX int, offsetY int) image.Image {
	rgba := image.NewRGBA(shadowBorder.Bounds())
	draw.Draw(rgba, shadowBorder.Bounds(), shadowBorder, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, img.Bounds().Add(image.Pt(offsetX, offsetY)), img, image.ZP, draw.Over)
	return rgba.SubImage(rgba.Rect)
}

// bgLog is just a small helper function for logging in the biasgame
func bgLog() *logrus.Entry {
	return cache.GetLogger().WithField("module", "biasgame")
}

// getBiasGameCache
func getBiasGameCache(key string, data interface{}) error {
	// get cache with given key
	cacheResult, err := cache.GetRedisClient().Get(fmt.Sprintf("robyul2-discord:biasgame:%s", key)).Bytes()
	if err != nil || err == redis.Nil {
		return err
	}

	// if the datas type is already []byte then set it to cache instead of unmarshal
	switch data.(type) {
	case []byte:
		data = cacheResult
		return nil
	}

	err = json.Unmarshal(cacheResult, data)
	return err
}

// setBiasGameCache
func setBiasGameCache(key string, data interface{}, time time.Duration) error {
	marshaledData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = cache.GetRedisClient().Set(fmt.Sprintf("robyul2-discord:biasgame:%s", key), marshaledData, time).Result()
	return err
}

// delBiasGameCache
func delBiasGameCache(keys ...string) {
	for _, key := range keys {

		cache.GetRedisClient().Del(fmt.Sprintf("robyul2-discord:biasgame:%s", key)).Result()
	}
}

// sendPagedEmbedOfImages takes the given image []byte and sends them in a paged embed
func sendPagedEmbedOfImages(msg *discordgo.Message, imagesToSend []idols.IdolImage, displayObjectIds bool, authorName, description string) {
	positionMap := []string{"Top Left", "Top Right", "Bottom Left", "Bottom Right"}

	// create images embed message
	imagesMessage := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Description: description,
			Color:       0x0FADED,
			Author: &discordgo.MessageEmbedAuthor{
				Name: authorName,
			},
			Image:  &discordgo.MessageEmbedImage{},
			Fields: []*discordgo.MessageEmbedField{},
		},
		Files: []*discordgo.File{},
	}

	// loop through images, make a 2x2 collage and set it as a file
	var images [][]byte
	for i, img := range imagesToSend {
		images = append(images, img.GetImgBytes())

		if displayObjectIds {

			imagesMessage.Embed.Fields = append(imagesMessage.Embed.Fields, &discordgo.MessageEmbedField{
				Name:  positionMap[i%4],
				Value: img.ObjectName,
			})
		}

		// one page should display 4 images
		if (i+1)%4 == 0 {

			// make collage and set the image as a file in the embed
			collageBytes := helpers.CollageFromBytes(images, []string{}, 300, 300, 150, 150, helpers.DISCORD_DARK_THEME_BACKGROUND_HEX)
			imagesMessage.Files = append(imagesMessage.Files, &discordgo.File{
				Name:   fmt.Sprintf("image%d.png", i),
				Reader: bytes.NewReader(collageBytes),
			})

			// reset images array
			images = make([][]byte, 0)
		}
	}

	// check for any left over images
	if len(images) > 0 {
		// make collage and set the image as a file in the embed
		collageBytes := helpers.CollageFromBytes(images, []string{}, 300, 300, 150, 150, helpers.DISCORD_DARK_THEME_BACKGROUND_HEX)
		imagesMessage.Files = append(imagesMessage.Files, &discordgo.File{
			Name:   fmt.Sprintf("image%d.png", len(imagesMessage.Files)+1),
			Reader: bytes.NewReader(collageBytes),
		})
	}

	// send paged embed
	helpers.SendPagedImageMessage(msg, imagesMessage, 4)
}

// <3
func getRandomNayoungEmoji() string {
	nayoungEmojiArray := []string{
		":nayoungthumbsup:430592739839705091",
		":nayoungsalute:430592737340030979",
		":nayounghype:430592740066066433",
		":nayoungheart6:430592739868934164",
		":nayoungheart2:430592737004224514",
		":nayoungheart:430592736496713738",
		":nayoungok:424683077793611777",
		"a:anayoungminnie:430592552610299924",
	}

	randomIndex := rand.Intn(len(nayoungEmojiArray))
	return nayoungEmojiArray[randomIndex]
}

// checks if the error is a permissions error and notifies the user
func checkPermissionError(err error, channelID string) bool {
	if err == nil {
		return false
	}

	// check if error is a permissions error
	if err, ok := err.(*discordgo.RESTError); ok && err.Message != nil {
		if err.Message.Code == discordgo.ErrCodeMissingPermissions {
			return true
		}
	}
	return false
}

// getSemiRandomIdolImage will check the given map to see if the given idol exists,
//   if they do it will return the image at the given index
//   if not it will return a random image
func getSemiRandomIdolImage(idol *idols.Idol, gameImageIndex *map[string]int) image.Image {
	var imageIndex int

	// check if a random image for the idol has already been chosen for this game
	//  also make sure that biasimages array contains the index. it may have been changed due to a refresh
	if imagePos, ok := (*gameImageIndex)[idol.NameAndGroup]; ok && len(idol.BiasImages) > imagePos {
		imageIndex = imagePos
	} else {
		imageIndex = rand.Intn(len(idol.BiasImages))
		(*gameImageIndex)[idol.NameAndGroup] = imageIndex
	}

	img, _, err := image.Decode(bytes.NewReader(idol.BiasImages[imageIndex].GetResizeImgBytes(IMAGE_RESIZE_HEIGHT)))
	helpers.Relax(err)
	return img
}
