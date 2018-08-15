package nugugame

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"os"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis"
	"github.com/nfnt/resize"
	"github.com/sirupsen/logrus"
)

var shadowBorder image.Image

// log is just a small helper function for logging in this module
func log() *logrus.Entry {
	return cache.GetLogger().WithField("module", "nugugame")
}

// getModuleCache for easily getting redis cache specific to this module
func getModuleCache(key string, data interface{}) error {
	// get cache with given key
	cacheResult, err := cache.GetRedisClient().Get(fmt.Sprintf("robyul2-discord:nugugame:%s", key)).Bytes()
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

// setModuleCache for easily setting redis cache specific to this module
func setModuleCache(key string, data interface{}, time time.Duration) error {
	marshaledData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = cache.GetRedisClient().Set(fmt.Sprintf("robyul2-discord:nugugame:%s", key), marshaledData, time).Result()
	return err
}

// delModuleCache for easily setting redis cache specific to this module
func delModuleCache(key string) {
	go func() {
		helpers.Recover()

		cache.GetRedisClient().Del(fmt.Sprintf("robyul2-discord:nugugame:%s", key))
	}()
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

// giveImageShadowBorder give the round image a shadow border
func giveImageShadowBorder(img image.Image, offsetX int, offsetY int) image.Image {
	rgba := image.NewRGBA(shadowBorder.Bounds())
	draw.Draw(rgba, shadowBorder.Bounds(), shadowBorder, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, img.Bounds().Add(image.Pt(offsetX, offsetY)), img, image.ZP, draw.Over)
	return rgba.SubImage(rgba.Rect)
}

// loadMiscImages handles loading other images besides the idol images
func loadMiscImages() {

	validMiscImages := []string{
		"shadow-border.png",
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
		case "shadow-border.png":
			shadowBorder = resize.Resize(0, NUGUGAME_IMAGE_RESIZE_HEIGHT+40, img, resize.Lanczos3)
		}
	}
}
