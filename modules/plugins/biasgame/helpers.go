package biasgame

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"regexp"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/go-redis/redis"
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

	json.Unmarshal(cacheResult, data)
	return nil
}

// setBiasGameCache
func setBiasGameCache(key string, data interface{}, time time.Duration) error {
	marshaledData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	cache.GetRedisClient().Set(fmt.Sprintf("robyul2-discord:biasgame:%s", key), marshaledData, time)
	return nil
}

// delBiasGameCache
func delBiasGameCache(keys ...string) {
	for _, key := range keys {

		cache.GetRedisClient().Del(fmt.Sprintf("robyul2-discord:biasgame:%s", key))
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
	for _, bias := range allBiasChoices {
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
