package rest

import (
	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
)

func getGuildFeatures(guildID string) (features models.Rest_Guild_Features) {
	cacheCodec := cache.GetRedisCacheCodec()

	var err error

	var featureLevels_Badges models.Rest_Feature_Levels_Badges
	key := fmt.Sprintf(models.Redis_Key_Feature_Levels_Badges, guildID)
	if err = cacheCodec.Get(key, &featureLevels_Badges); err != nil {
		featureLevels_Badges = models.Rest_Feature_Levels_Badges{
			Count: 0,
		}
	}

	var featureRandomPictures models.Rest_Feature_RandomPictures
	key = fmt.Sprintf(models.Redis_Key_Feature_RandomPictures, guildID)
	if err = cacheCodec.Get(key, &featureRandomPictures); err != nil {
		featureRandomPictures = models.Rest_Feature_RandomPictures{
			Count: 0,
		}
	}
	guildSettings := helpers.GuildSettingsGetCached(guildID)
	featureChatlog := models.Rest_Feature_Chatlog{Enabled: true}
	if guildSettings.ChatlogDisabled {
		featureChatlog.Enabled = false
	}

	featureEventlog := models.Rest_Feature_Eventlog{Enabled: true}
	if guildSettings.EventlogDisabled {
		featureEventlog.Enabled = false
	}

	var featureVanityInvite models.Rest_Feature_VanityInvite
	vanityInvite, _ := helpers.GetVanityUrlByGuildID(guildID)
	featureVanityInvite.VanityInviteName = vanityInvite.VanityName

	var featureModules []models.Rest_Feature_Module
	featureModules = make([]models.Rest_Feature_Module, 0)
	disabledModules := helpers.GetDisabledModules(guildID)
NextModule:
	for _, module := range helpers.Modules {
		for _, disabledModule := range disabledModules {
			if disabledModule == module.Permission {
				continue NextModule
			}
		}
		featureModules = append(featureModules, models.Rest_Feature_Module{
			Name: helpers.GetModuleNameById(module.Permission),
			ID:   module.Permission,
		})
	}

	return models.Rest_Guild_Features{
		Levels_Badges:  featureLevels_Badges,
		RandomPictures: featureRandomPictures,
		Chatlog:        featureChatlog,
		VanityInvite:   featureVanityInvite,
		Modules:        featureModules,
		Eventlog:       featureEventlog,
	}
}

func getGuildSettings(guildID, userID string) (settings models.Rest_Settings) {
	settings = models.Rest_Settings{
		Strings: make([]models.Rest_Setting_String, 0),
	}

	return
}

func setGuildStringSetting(guildID, userID, key string, values []string) (err error) {
	return
}
