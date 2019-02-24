package idols

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

var alphaNumericRegex *regexp.Regexp

// bgLog is just a small helper function for logging in this module
func log() *logrus.Entry {
	return cache.GetLogger().WithField("module", "idols")
}

// getIdolCache for easily getting redis cache specific to idols
func getModuleCache(key string, data interface{}) error {
	// get cache with given key
	cacheResult, err := cache.GetRedisClient().Get(fmt.Sprintf("robyul2-discord:idols:%s", key)).Bytes()
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

// setIdolsCache for easily setting redis cache specific to idols
func setModuleCache(key string, data interface{}, time time.Duration) error {
	marshaledData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = cache.GetRedisClient().Set(fmt.Sprintf("robyul2-discord:idols:%s", key), marshaledData, time).Result()
	return err
}

// alphaNumericCompare does a case insensitive alpha numeric comparison of two input
func alphaNumericCompare(input1, input2 string) bool {
	regInput1 := strings.ToLower(alphaNumericRegex.ReplaceAllString(input1, ""))
	regInput2 := strings.ToLower(alphaNumericRegex.ReplaceAllString(input2, ""))
	return regInput1 == regInput2
}

// sendPagedEmbedOfImages takes the given image []byte and sends them in a paged embed
func sendPagedEmbedOfImages(msg *discordgo.Message, imagesToSend []IdolImage, displayObjectIds bool, authorName, description string) {
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
		images = append(images, img.GetResizeImgBytes(IMAGE_RESIZE_HEIGHT))

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

func compileGameStats(records map[string]int) (map[int][]string, []int) {
	// use map of counts to compile a new map of [unique occurence amounts]Names
	var uniqueCounts []int
	compiledData := make(map[int][]string)
	for k, v := range records {
		// store unique counts so the map can be "sorted"
		if _, ok := compiledData[v]; !ok {
			uniqueCounts = append(uniqueCounts, v)
		}

		compiledData[v] = append(compiledData[v], k)
	}

	// sort biggest to smallest
	sort.Sort(sort.Reverse(sort.IntSlice(uniqueCounts)))

	return compiledData, uniqueCounts
}
