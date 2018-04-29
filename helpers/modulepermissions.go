package helpers

import (
	"sync"

	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"
)

type ModulePermissionsModuleInfo struct {
	Names      []string
	Permission models.ModulePermissionsModule
}

var (
	modulePermissionsCache    []models.ModulePermissionEntry
	modulePermissionCacheLock sync.Mutex
)

const (
	ModulePermStats              models.ModulePermissionsModule = "stats"          // stats.go, uptime.go
	ModulePermTranslator         models.ModulePermissionsModule = "translator"     // translator.go
	ModulePermUrban              models.ModulePermissionsModule = "urban"          // urbandict.go
	ModulePermWeather            models.ModulePermissionsModule = "weather"        // weather.go
	ModulePermVLive              models.ModulePermissionsModule = "vlive"          // vlive.go
	ModulePermInstagram          models.ModulePermissionsModule = "instagram"      // instagram/
	ModulePermFacebook           models.ModulePermissionsModule = "facebook"       // facebook.go
	ModulePermWolframAlpha       models.ModulePermissionsModule = "wolframalpha"   // wolframalpha.go
	ModulePermLastFm             models.ModulePermissionsModule = "lastfm"         // lastfm.go
	ModulePermTwitch             models.ModulePermissionsModule = "twitch"         // twitch.go
	ModulePermCharts             models.ModulePermissionsModule = "charts"         // charts.go
	ModulePermChoice             models.ModulePermissionsModule = "choice"         // choice.go
	ModulePermOsu                models.ModulePermissionsModule = "osu"            // osu.go
	ModulePermReminders          models.ModulePermissionsModule = "reminders"      // reminders.go
	ModulePermGfycat             models.ModulePermissionsModule = "gfycat"         // gfycat.go
	ModulePermRandomPictures     models.ModulePermissionsModule = "randompictures" // randompictures.go
	ModulePermYouTube            models.ModulePermissionsModule = "youtube"        // youtube/
	ModulePermSpoiler            models.ModulePermissionsModule = "spoiler"        // spoiler.go
	ModulePermAnimals            models.ModulePermissionsModule = "animals"        // random_cat.go, dog.go
	ModulePermGames              models.ModulePermissionsModule = "games"          // rps.go, biasgame/
	ModulePermDig                models.ModulePermissionsModule = "dig"            // dig.go
	ModulePermStreamable         models.ModulePermissionsModule = "streamable"     // streamable.go
	ModulePermLyrics             models.ModulePermissionsModule = "lyrcis"         // lyrics.go
	ModulePermMisc               models.ModulePermissionsModule = "misc"
	ModulePermReddit             models.ModulePermissionsModule = "reddit"             // reddit.go
	ModulePermColor              models.ModulePermissionsModule = "color"              // color.go
	ModulePermSteam              models.ModulePermissionsModule = "steam"              // dog.go
	ModulePermGoogle             models.ModulePermissionsModule = "google"             // google/
	ModulePermWhois              models.ModulePermissionsModule = "whois"              // whois.go
	ModulePermIsup               models.ModulePermissionsModule = "isup"               // isup.go
	ModulePermLevels             models.ModulePermissionsModule = "levels"             // levels.go
	ModulePermCustomCommands     models.ModulePermissionsModule = "customcommands"     // customcommands.go
	ModulePermReactionPolls      models.ModulePermissionsModule = "reactionpolls"      // reactionpolls.go
	ModulePermTwitter            models.ModulePermissionsModule = "twitter"            // twitter.go
	ModulePermStarboard          models.ModulePermissionsModule = "starboard"          // starboard.go
	ModulePermAutoRole           models.ModulePermissionsModule = "autorole"           // autorole.go
	ModulePermBias               models.ModulePermissionsModule = "bias"               // bias.go
	ModulePermDiscordmoney       models.ModulePermissionsModule = "discordmoney"       // discordmoney.go
	ModulePermGallery            models.ModulePermissionsModule = "gallery"            // gallery.go
	ModulePermGuildAnnouncements models.ModulePermissionsModule = "guildannouncements" // guildannouncements.go
	ModulePermMirror             models.ModulePermissionsModule = "mirror"             // mirror.go
	ModulePermMod                models.ModulePermissionsModule = "mod"                // mod.go
	ModulePermNotifications      models.ModulePermissionsModule = "notifications"      // notifications.go
	ModulePermNuke               models.ModulePermissionsModule = "nuke"               // nuke.go
	ModulePermPersistency        models.ModulePermissionsModule = "persistency"        // persistency.go
	ModulePermPing               models.ModulePermissionsModule = "ping"               // ping.go
	ModulePermTroublemaker       models.ModulePermissionsModule = "troublemaker"       // troublemaker.go
	ModulePermVanityInvite       models.ModulePermissionsModule = "vanityinvite"       // vanityinvite.go
	ModulePerm8ball              models.ModulePermissionsModule = "8ball"              // 8ball.go
	ModulePermFeedback           models.ModulePermissionsModule = "feedback"           // feedback.go
	ModulePermEmbedPost          models.ModulePermissionsModule = "embed"              // embedpost.go
	ModulePermEventlog           models.ModulePermissionsModule = "eventlog"           // eventlog/
	ModulePermCrypto             models.ModulePermissionsModule = "crypto"             // crypto.go
	ModulePermImgur              models.ModulePermissionsModule = "imgur"              // imgur.go

	ModulePermAllPlaceholder models.ModulePermissionsModule = "all"
)

var (
	ModulePermAll = []models.ModulePermissionsModule{ModulePermStats, ModulePermTranslator, ModulePermUrban, ModulePermWeather, ModulePermVLive,
		ModulePermInstagram, ModulePermFacebook, ModulePermWolframAlpha, ModulePermLastFm, ModulePermTwitter,
		ModulePermTwitch, ModulePermCharts, ModulePermChoice, ModulePermOsu, ModulePermReminders,
		ModulePermGfycat, ModulePermRandomPictures, ModulePermYouTube, ModulePermSpoiler, ModulePermAnimals,
		ModulePermGames, ModulePermDig, ModulePermStreamable, ModulePermLyrics, ModulePermMisc, ModulePermReddit,
		ModulePermColor, ModulePermSteam, ModulePermGoogle, ModulePermWhois, ModulePermIsup, ModulePermLevels,
		ModulePermCustomCommands, ModulePermReactionPolls, ModulePermTwitter, ModulePermStarboard,
		ModulePermAutoRole, ModulePermBias, ModulePermDiscordmoney, ModulePermGallery,
		ModulePermGuildAnnouncements, ModulePermMirror, ModulePermMirror, ModulePermMod, ModulePermNotifications,
		ModulePermNuke, ModulePermPersistency, ModulePermPing, ModulePermTroublemaker, ModulePermVanityInvite,
		ModulePerm8ball, ModulePermFeedback, ModulePermEmbedPost, ModulePermEventlog, ModulePermCrypto, ModulePermImgur}
)

var (
	Modules = []ModulePermissionsModuleInfo{
		{Names: []string{"stats"}, Permission: ModulePermStats},
		{Names: []string{"translator"}, Permission: ModulePermTranslator},
		{Names: []string{"urbandict", "urban"}, Permission: ModulePermUrban},
		{Names: []string{"weather"}, Permission: ModulePermWeather},
		{Names: []string{"vlive"}, Permission: ModulePermVLive},
		{Names: []string{"instagram"}, Permission: ModulePermInstagram},
		{Names: []string{"facebook"}, Permission: ModulePermFacebook},
		{Names: []string{"wolframalpha", "wolfram"}, Permission: ModulePermWolframAlpha},
		{Names: []string{"lastfm"}, Permission: ModulePermLastFm},
		{Names: []string{"twitch"}, Permission: ModulePermTwitch},
		{Names: []string{"charts"}, Permission: ModulePermCharts},
		{Names: []string{"choice"}, Permission: ModulePermChoice},
		{Names: []string{"osu"}, Permission: ModulePermOsu},
		{Names: []string{"reminders"}, Permission: ModulePermReminders},
		{Names: []string{"gfycat"}, Permission: ModulePermGfycat},
		{Names: []string{"randompictures"}, Permission: ModulePermRandomPictures},
		{Names: []string{"youtube"}, Permission: ModulePermYouTube},
		{Names: []string{"spoiler"}, Permission: ModulePermSpoiler},
		{Names: []string{"animals", "cat", "randomcat", "dog"}, Permission: ModulePermAnimals},
		{Names: []string{"games", "rps", "biasgame"}, Permission: ModulePermGames},
		{Names: []string{"dig"}, Permission: ModulePermDig},
		{Names: []string{"streamable"}, Permission: ModulePermStreamable},
		{Names: []string{"lyrics"}, Permission: ModulePermLyrics},
		{Names: []string{"misc"}, Permission: ModulePermMisc},
		{Names: []string{"reddit"}, Permission: ModulePermReddit},
		{Names: []string{"color"}, Permission: ModulePermColor},
		{Names: []string{"steam"}, Permission: ModulePermSteam},
		{Names: []string{"google"}, Permission: ModulePermGoogle},
		{Names: []string{"whois"}, Permission: ModulePermWhois},
		{Names: []string{"isup"}, Permission: ModulePermIsup},
		{Names: []string{"levels"}, Permission: ModulePermLevels},
		{Names: []string{"customcommands"}, Permission: ModulePermCustomCommands},
		{Names: []string{"reactionpolls"}, Permission: ModulePermReactionPolls},
		{Names: []string{"twitter"}, Permission: ModulePermTwitter},
		{Names: []string{"starboard"}, Permission: ModulePermStarboard},
		{Names: []string{"autorole"}, Permission: ModulePermAutoRole},
		{Names: []string{"bias"}, Permission: ModulePermBias},
		{Names: []string{"discordmoney"}, Permission: ModulePermDiscordmoney},
		{Names: []string{"gallery"}, Permission: ModulePermGallery},
		{Names: []string{"serverannouncements", "guildannouncements"}, Permission: ModulePermGuildAnnouncements},
		{Names: []string{"mirror"}, Permission: ModulePermMirror},
		{Names: []string{"mod", "names"}, Permission: ModulePermMod},
		{Names: []string{"notifications"}, Permission: ModulePermNotifications},
		{Names: []string{"nuke"}, Permission: ModulePermNuke},
		{Names: []string{"persistency"}, Permission: ModulePermPersistency},
		{Names: []string{"ping"}, Permission: ModulePermPing},
		{Names: []string{"troublemaker"}, Permission: ModulePermTroublemaker},
		{Names: []string{"custominvite", "vanityinvite"}, Permission: ModulePermVanityInvite},
		{Names: []string{"8ball"}, Permission: ModulePerm8ball},
		{Names: []string{"feedback"}, Permission: ModulePermFeedback},
		{Names: []string{"embed", "embedpost"}, Permission: ModulePermEmbedPost},
		{Names: []string{"eventlog"}, Permission: ModulePermEventlog},
		{Names: []string{"crypto"}, Permission: ModulePermCrypto},
		{Names: []string{"imgur"}, Permission: ModulePermImgur},
	}
)

func RefreshModulePermissionsCache() (err error) {
	modulePermissionCacheLock.Lock()
	defer modulePermissionCacheLock.Unlock()
	err = MDbIter(MdbCollection(models.ModulePermissionsTable).Find(nil)).All(&modulePermissionsCache)
	return err
}

func ModuleIsAllowed(channelID, msgID, userID string, module models.ModulePermissionsModule) (isAllowed bool) {
	isAllowed = ModuleIsAllowedSilent(channelID, msgID, userID, module)

	if !isAllowed && msgID != "" {
		go AddNoPermissionsReaction(channelID, msgID)
	}
	return isAllowed
}

func ModuleIsAllowedSilent(channelID, msgID, userID string, module models.ModulePermissionsModule) (isAllowed bool) {
	channel, err := GetChannelWithoutApi(channelID)
	if err != nil {
		cache.GetLogger().WithField("module", "modulepermissions").Errorf(
			"failed to get channel for ModuleIsAllowedSilent message #%s channel #%s user #%s module %s: %s",
			msgID, channelID, userID, GetModuleNameById(module), err.Error(),
		)
		return true
	}

	user, err := GetGuildMemberWithoutApi(channel.GuildID, userID)
	if err != nil {
		cache.GetLogger().WithField("module", "modulepermissions").Errorf(
			"failed to get guild member for ModuleIsAllowedSilent message #%s channel #%s user #%s module %s: %s",
			msgID, channelID, userID, GetModuleNameById(module), err.Error(),
		)
		return true
	}

	guild, err := GetGuildWithoutApi(channel.GuildID)
	if err != nil {
		cache.GetLogger().WithField("module", "modulepermissions").Errorf(
			"failed to get guild for ModuleIsAllowedSilent message #%s channel #%s user #%s module %s: %s",
			msgID, channelID, userID, GetModuleNameById(module), err.Error(),
		)
		return true
	}

	userRoles := user.Roles

	var everyoneRoleID string
	for _, role := range guild.Roles {
		if role.Name == "@everyone" {
			everyoneRoleID = role.ID
		}
	}
	if everyoneRoleID != "" {
		userRoles = append(userRoles, everyoneRoleID)
	}

	var checkParent bool
	if ChannelPermissionsInSync(channelID) {
		checkParent = true
	}

	// allowed role > denied role > allowed channel > denied channel > allowed parent channel (if in sync) > denied parent channel (if in sync)

	// allowed role
	for _, userRoleID := range userRoles {
		if GetAllowedForRole(channel.GuildID, userRoleID) != nil {
			if ModulePermissionsContain(GetAllowedForRole(channel.GuildID, userRoleID), module) ||
				ModulePermissionsContain(GetAllowedForRole(channel.GuildID, userRoleID), ModulePermAllPlaceholder) {
				cache.GetLogger().WithField("module", "modulepermissions").Infof(
					"allowed command by role message #%s channel #%s user #%s module %s",
					msgID, channelID, userID, GetModuleNameById(module),
				)
				return true
			}
		}
	}

	// denied role
	for _, userRoleID := range userRoles {
		if GetDeniedForRole(channel.GuildID, userRoleID) != nil {
			if ModulePermissionsContain(GetDeniedForRole(channel.GuildID, userRoleID), module) ||
				ModulePermissionsContain(GetDeniedForRole(channel.GuildID, userRoleID), ModulePermAllPlaceholder) {
				cache.GetLogger().WithField("module", "modulepermissions").Infof(
					"denied command by role message #%s channel #%s user #%s module %s",
					msgID, channelID, userID, GetModuleNameById(module),
				)
				return false
			}
		}
	}

	// allowed channel
	if GetAllowedForChannel(channel.GuildID, channelID) != nil {
		if ModulePermissionsContain(GetAllowedForChannel(channel.GuildID, channelID), module) ||
			ModulePermissionsContain(GetAllowedForChannel(channel.GuildID, channelID), ModulePermAllPlaceholder) {
			cache.GetLogger().WithField("module", "modulepermissions").Infof(
				"allowed command by channel message #%s channel #%s user #%s module %s",
				msgID, channelID, userID, GetModuleNameById(module),
			)
			return true
		}
	}

	// denied channel
	if GetDeniedForChannel(channel.GuildID, channelID) != nil {
		if ModulePermissionsContain(GetDeniedForChannel(channel.GuildID, channelID), module) ||
			ModulePermissionsContain(GetDeniedForChannel(channel.GuildID, channelID), ModulePermAllPlaceholder) {
			cache.GetLogger().WithField("module", "modulepermissions").Infof(
				"denied command by channel message #%s channel #%s user #%s module %s",
				msgID, channelID, userID, GetModuleNameById(module),
			)
			return false
		}
	}

	// allowed parent channel (if in sync)
	if checkParent {
		if GetAllowedForChannel(channel.GuildID, channel.ParentID) != nil {
			if ModulePermissionsContain(GetAllowedForChannel(channel.GuildID, channel.ParentID), module) ||
				ModulePermissionsContain(GetAllowedForChannel(channel.GuildID, channel.ParentID), ModulePermAllPlaceholder) {
				cache.GetLogger().WithField("module", "modulepermissions").Infof(
					"allowed command by parent channel message #%s channel #%s user #%s module %s",
					msgID, channelID, userID, GetModuleNameById(module),
				)
				return true
			}
		}
	}

	// denied parent channel (if in sync)
	if checkParent {
		if GetDeniedForChannel(channel.GuildID, channel.ParentID) != nil {
			if ModulePermissionsContain(GetDeniedForChannel(channel.GuildID, channel.ParentID), module) ||
				ModulePermissionsContain(GetDeniedForChannel(channel.GuildID, channel.ParentID), ModulePermAllPlaceholder) {
				cache.GetLogger().WithField("module", "modulepermissions").Infof(
					"denied command by parent channel message #%s channel #%s user #%s module %s",
					msgID, channelID, userID, GetModuleNameById(module),
				)
				return false
			}
		}
	}

	return true
}

func GetModuleNameById(ids ...models.ModulePermissionsModule) (name string) {
	if ModulePermissionsContain(ids, ModulePermAllPlaceholder) {
		return "all modules"
	}
	for _, id := range ids {
		for _, module := range Modules {
			if module.Permission == id {
				if len(module.Names) > 0 {
					name += module.Names[0] + ", "
				}
			}
		}
	}
	name = strings.TrimRight(name, ", ")
	return name
}

func GetDisabledModules(guildID string) (disabledModules []models.ModulePermissionsModule) {
	guild, err := GetGuild(guildID)
	if err != nil {
		return disabledModules
	}

	var everyoneRoleID string
	for _, role := range guild.Roles {
		if role.Name == "@everyone" {
			everyoneRoleID = role.ID
		}
	}
	if everyoneRoleID == "" {
		return disabledModules
	}

NextModule:
	for _, module := range Modules {
		if GetDeniedForRole(guildID, everyoneRoleID) != nil {
			if ModulePermissionsContain(GetDeniedForRole(guildID, everyoneRoleID), module.Permission) {
				// is denied for everyone
				for _, role := range guild.Roles {
					if GetAllowedForRole(guildID, role.ID) != nil {
						if ModulePermissionsContain(GetAllowedForRole(guildID, role.ID), module.Permission) {
							continue NextModule
						}
					}
				}

				disabledModules = append(disabledModules, module.Permission)
			}
		}
	}

	return disabledModules
}

func SetAllowedForChannel(guildID, channelID string, newPermissions []models.ModulePermissionsModule) (err error) {
	entry := findModulePermissionsEntry(guildID, "channel", channelID)
	entry.AllowedModules = newPermissions
	err = setModulePermissionsEntry(entry)
	return err
}

func SetAllowedForRole(guildID, roleID string, newPermissions []models.ModulePermissionsModule) (err error) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	entry.AllowedModules = newPermissions
	err = setModulePermissionsEntry(entry)
	return err
}

func SetDeniedForChannel(guildID, channelID string, newPermissions []models.ModulePermissionsModule) (err error) {
	entry := findModulePermissionsEntry(guildID, "channel", channelID)
	entry.DeniedModules = newPermissions
	err = setModulePermissionsEntry(entry)
	return err
}

func SetDeniedForRole(guildID, roleID string, newPermissions []models.ModulePermissionsModule) (err error) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	entry.DeniedModules = newPermissions
	err = setModulePermissionsEntry(entry)
	return err
}

func GetAllowedForChannel(guildID, channelID string) (permissions []models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "channel", channelID)
	if entry.AllowedModules != nil {
		if ModulePermissionsContain(entry.AllowedModules, ModulePermAllPlaceholder) {
			return append(ModulePermAll, ModulePermAllPlaceholder)
		}
		return entry.AllowedModules
	}
	return nil
}

func GetAllowedForRole(guildID, roleID string) (permissions []models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	if entry.AllowedModules != nil {
		if ModulePermissionsContain(entry.AllowedModules, ModulePermAllPlaceholder) {
			return append(ModulePermAll, ModulePermAllPlaceholder)
		}
		return entry.AllowedModules
	}
	return nil
}

func GetDeniedForChannel(guildID, channelID string) (permissions []models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "channel", channelID)
	if entry.DeniedModules != nil {
		if ModulePermissionsContain(entry.AllowedModules, ModulePermAllPlaceholder) {
			return append(ModulePermAll, ModulePermAllPlaceholder)
		}
		return entry.DeniedModules
	}
	return nil
}

func GetDeniedForRole(guildID, roleID string) (permissions []models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	if entry.DeniedModules != nil {
		if ModulePermissionsContain(entry.AllowedModules, ModulePermAllPlaceholder) {
			return append(ModulePermAll, ModulePermAllPlaceholder)
		}
		return entry.DeniedModules
	}
	return nil
}

func findModulePermissionsEntry(guildID, permType, targetID string) (entry models.ModulePermissionEntry) {
	if modulePermissionsCache != nil {
		modulePermissionCacheLock.Lock()
		defer modulePermissionCacheLock.Unlock()
		for _, module := range modulePermissionsCache {
			if module.GuildID == guildID && module.Type == permType && module.TargetID == targetID {
				return module
			}
		}
		entry = models.GetDefaultModulePermission()
		entry.GuildID = guildID
		entry.Type = permType
		entry.TargetID = targetID
		return entry
	}
	err := MdbOne(
		MdbCollection(models.ModulePermissionsTable).Find(bson.M{"guildid": guildID, "type": permType, "targetid": targetID}),
		&entry,
	)
	if err != nil {
		entry = models.GetDefaultModulePermission()
		entry.GuildID = guildID
		entry.Type = permType
		entry.TargetID = targetID
		return entry
	}

	return entry
}

func GetModulePermissionEntries(guildID string) (entries []models.ModulePermissionEntry) {
	if modulePermissionsCache != nil {
		modulePermissionCacheLock.Lock()
		defer modulePermissionCacheLock.Unlock()
		for _, module := range modulePermissionsCache {
			if module.GuildID == guildID {
				entries = append(entries, module)
			}
		}
		return entries
	}

	_ = MDbIter(MdbCollection(models.ModulePermissionsTable).Find(bson.M{"guildid": guildID})).All(&entries)
	return entries
}

func setModulePermissionsEntry(entry models.ModulePermissionEntry) (err error) {
	if entry.ID != "" {
		err = MDbUpsertID(
			models.ModulePermissionsTable,
			entry.ID,
			entry,
		)
		go func() {
			defer Recover()

			err := RefreshModulePermissionsCache()
			Relax(err)
		}()
		return err
	}
	_, err = MDbInsert(
		models.ModulePermissionsTable,
		entry,
	)
	go func() {
		defer Recover()

		err := RefreshModulePermissionsCache()
		Relax(err)
	}()
	return err
}

func ModulePermissionsContain(list []models.ModulePermissionsModule, entries ...models.ModulePermissionsModule) (contains bool) {
	for _, entry := range entries {
		contains := false
		for _, listEntry := range list {
			if listEntry == entry {
				contains = true
			}
		}
		if !contains {
			return false
		}
	}
	return true
}

func ModulePermissionsWithout(list []models.ModulePermissionsModule, without ...models.ModulePermissionsModule) (newList []models.ModulePermissionsModule) {
	for _, listEntry := range list {
		if !ModulePermissionsContain(without, listEntry) {
			newList = append(newList, listEntry)
		}
	}
	return newList
}
