package biasgame

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis"
	"github.com/nfnt/resize"
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

// getMatchingIdolAndGroup will do a loose comparison of the name and group passed to the ones that already exist
//  1st return is true if group exists
//  2nd return is true if idol exists in the group
//  3rd will be a reference to the matching idol
func getMatchingIdolAndGroup(searchGroup, searchName string) (bool, bool, *biasChoice) {
	groupMatch := false
	nameMatch := false
	var matchingBiasChoice *biasChoice

	// create map of group => idols in group
	groupIdolMap := make(map[string][]*biasChoice)
	for _, bias := range getAllBiases() {
		groupIdolMap[bias.GroupName] = append(groupIdolMap[bias.GroupName], bias)
	}

	// check if the group suggested matches a current group. do loose comparison
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	for k, v := range groupIdolMap {
		curGroup := strings.ToLower(reg.ReplaceAllString(k, ""))
		sugGroup := strings.ToLower(reg.ReplaceAllString(searchGroup, ""))

		// if groups match, set the suggested group to the current group
		if curGroup == sugGroup {
			groupMatch = true

			// check if the idols name matches
			for _, idol := range v {
				curName := strings.ToLower(reg.ReplaceAllString(idol.BiasName, ""))
				sugName := strings.ToLower(reg.ReplaceAllString(searchName, ""))

				if curName == sugName {
					nameMatch = true
					matchingBiasChoice = idol
					break
				}
			}
			break
		}
	}

	return groupMatch, nameMatch, matchingBiasChoice
}

// sendPagedEmbedOfImages takes the given image []byte and sends them in a paged embed
func sendPagedEmbedOfImages(msg *discordgo.Message, imagesToSend [][]byte, authorName, description string) {

	// create images embed message
	imagesMessage := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Description: description,
			Color:       0x0FADED,
			Author: &discordgo.MessageEmbedAuthor{
				Name: authorName,
			},
			Image: &discordgo.MessageEmbedImage{},
		},
		Files: []*discordgo.File{},
	}

	// loop through images, make a 2x2 collage and set it as a file
	var images [][]byte
	for i, imageBytes := range imagesToSend {
		images = append(images, imageBytes)

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
	helpers.SendPagedImageMessage(msg, imagesMessage)
}

// makeVSImage will make the image that shows for rounds in the biasgame
func makeVSImage(img1, img2 image.Image) image.Image {
	// resize images if needed
	if img1.Bounds().Dy() != IMAGE_RESIZE_HEIGHT || img2.Bounds().Dy() != IMAGE_RESIZE_HEIGHT {
		img1 = resize.Resize(0, IMAGE_RESIZE_HEIGHT, img1, resize.Lanczos3)
		img2 = resize.Resize(0, IMAGE_RESIZE_HEIGHT, img2, resize.Lanczos3)
	}

	// give shadow border
	img1 = giveImageShadowBorder(img1, 15, 15)
	img2 = giveImageShadowBorder(img2, 15, 15)

	// combind images
	img1 = helpers.CombineTwoImages(img1, versesImage)
	return helpers.CombineTwoImages(img1, img2)
}

// getAllBiases getter for all biases
func getAllBiases() []*biasChoice {
	allBiasesMutex.RLock()
	defer allBiasesMutex.RUnlock()

	if allBiasChoices == nil {
		return nil
	}

	return allBiasChoices
}

// setAllBiases setter for all biases
func setAllBiases(biases []*biasChoice) {
	allBiasesMutex.Lock()
	defer allBiasesMutex.Unlock()

	allBiasChoices = biases
}

// holds aliases for commands
func isCommandAlias(input, targetCommand string) bool {
	// if input is already the same as target command no need to check aliases
	if input == targetCommand {
		return true
	}

	var aliasMap = map[string]string{
		"images": "images",
		"image":  "images",
		"pic":    "images",
		"pics":   "images",
		"img":    "images",
		"imgs":   "images",

		"rankings": "rankings",
		"ranking":  "rankings",
		"rank":     "rankings",
		"ranks":    "rankings",

		"current": "current",
		"cur":     "current",

		"multi":       "multi",
		"multiplayer": "multi",

		"server-rankings": "server-rankings",
		"server-ranking":  "server-rankings",
		"server-ranks":    "server-rankings",
		"server-rank":     "server-rankings",
	}

	if attemptedCommand, ok := aliasMap[input]; ok {
		return attemptedCommand == targetCommand
	}

	return false
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
		"a:anayoungminnie:430592552610299924",
	}

	randomIndex := rand.Intn(len(nayoungEmojiArray))
	return nayoungEmojiArray[randomIndex]
}
