package helpers

import (
	"sync"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/gorethink/gorethink"
)

type ModulePermissionsModuleInfo struct {
	Names      []string
	Permission models.ModulePermissionsModule
}

var (
	modulePermissionsCache    []models.ModulePermission
	modulePermissionCacheLock sync.Mutex
)

const (
	ModulePermStats              models.ModulePermissionsModule = 1 << iota // stats.go, uptime.go
	ModulePermTranslator                                                    // translator.go
	ModulePermUrban                                                         // urbandict.go
	ModulePermWeather                                                       // weather.go
	ModulePermVLive                                                         // vlive.go
	ModulePermInstagram                                                     // instagram/
	ModulePermFacebook                                                      // facebook.go
	ModulePermWolframAlpha                                                  // wolframalpha.go
	ModulePermLastFm                                                        // lastfm.go
	ModulePermTwitch                                                        // twitch.go
	ModulePermCharts                                                        // charts.go
	ModulePermChoice                                                        // choice.go
	ModulePermOsu                                                           // osu.go
	ModulePermReminders                                                     // reminders.go
	ModulePermGfycat                                                        // gfycat.go
	ModulePermRandomPictures                                                // randompictures.go
	ModulePermYouTube                                                       // youtube/
	ModulePermSpoiler                                                       // spoiler.go
	ModulePermRandomCat                                                     // random_cat.go
	ModulePermRPS                                                           // rps.go
	ModulePermDig                                                           // dig.go
	ModulePermStreamable                                                    // streamable.go
	ModulePermLyrics                                                        // lyrics.go
	ModulePermNames                                                         // names.go
	ModulePermReddit                                                        // reddit.go
	ModulePermColor                                                         // color.go
	ModulePermDog                                                           // dog.go
	ModulePermGoogle                                                        // google/
	ModulePermWhois                                                         // whois.go
	ModulePermIsup                                                          // isup.go
	ModulePermLevels                                                        // levels.go
	ModulePermCustomCommands                                                // customcommands.go
	ModulePermReactionPolls                                                 // reactionpolls.go
	ModulePermTwitter                                                       // twitter.go
	ModulePermStarboard                                                     // starboard.go
	ModulePermAutoRole                                                      // autorole.go
	ModulePermBias                                                          // bias.go
	ModulePermDiscordmoney                                                  // discordmoney.go
	ModulePermGallery                                                       // gallery.go
	ModulePermGuildAnnouncements                                            // guildannouncements.go
	ModulePermMirror                                                        // mirror.go
	ModulePermMod                                                           // mod.go
	ModulePermNotifications                                                 // notifications.go
	ModulePermNuke                                                          // nuke.go
	ModulePermPersistency                                                   // persistency.go
	ModulePermPing                                                          // ping.go
	ModulePermTroublemaker                                                  // troublemaker.go
	ModulePermVanityInvite                                                  // vanityinvite.go
	ModulePerm8ball                                                         // 8ball.go
	ModulePermAllPlaceholder
	ModulePermFeedback  // feedback.go
	ModulePermEmbedPost // embedpost.go
	ModulePermEventlog  // eventlog/
	ModulePermCrypto    // crypto.go
	ModulePermImgur     // imgur.go

	ModulePermAll = ModulePermStats | ModulePermTranslator | ModulePermUrban | ModulePermWeather | ModulePermVLive |
		ModulePermInstagram | ModulePermFacebook | ModulePermWolframAlpha | ModulePermLastFm | ModulePermTwitter |
		ModulePermTwitch | ModulePermCharts | ModulePermChoice | ModulePermOsu | ModulePermReminders |
		ModulePermGfycat | ModulePermRandomPictures | ModulePermYouTube | ModulePermSpoiler | ModulePermRandomCat |
		ModulePermRPS | ModulePermDig | ModulePermStreamable | ModulePermLyrics | ModulePermNames | ModulePermReddit |
		ModulePermColor | ModulePermDog | ModulePermGoogle | ModulePermWhois | ModulePermIsup | ModulePermLevels |
		ModulePermCustomCommands | ModulePermReactionPolls | ModulePermTwitter | ModulePermStarboard |
		ModulePermAutoRole | ModulePermBias | ModulePermDiscordmoney | ModulePermGallery |
		ModulePermGuildAnnouncements | ModulePermMirror | ModulePermMirror | ModulePermMod | ModulePermNotifications |
		ModulePermNuke | ModulePermPersistency | ModulePermPing | ModulePermTroublemaker | ModulePermVanityInvite |
		ModulePerm8ball | ModulePermFeedback | ModulePermEmbedPost | ModulePermEventlog | ModulePermCrypto | ModulePermImgur
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
		{Names: []string{"cat", "randomcat"}, Permission: ModulePermRandomCat},
		{Names: []string{"rps"}, Permission: ModulePermRPS},
		{Names: []string{"dig"}, Permission: ModulePermDig},
		{Names: []string{"streamable"}, Permission: ModulePermStreamable},
		{Names: []string{"lyrics"}, Permission: ModulePermLyrics},
		{Names: []string{"names"}, Permission: ModulePermNames},
		{Names: []string{"reddit"}, Permission: ModulePermReddit},
		{Names: []string{"color"}, Permission: ModulePermColor},
		{Names: []string{"dog"}, Permission: ModulePermDog},
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
		{Names: []string{"mod"}, Permission: ModulePermMod},
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
	listCursor, err := gorethink.Table(models.ModulePermissionsTable).Run(GetDB())
	if err != nil {
		return err
	}
	defer listCursor.Close()
	modulePermissionCacheLock.Lock()
	defer modulePermissionCacheLock.Unlock()
	err = listCursor.All(&modulePermissionsCache)
	if err != nil {
		return err
	}

	return nil
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

	user, err := GetGuildMember(channel.GuildID, userID)
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
		if GetAllowedForRole(channel.GuildID, userRoleID)&module == module ||
			GetAllowedForRole(channel.GuildID, userRoleID)&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
			cache.GetLogger().WithField("module", "modulepermissions").Infof(
				"allowed command by role message #%s channel #%s user #%s module %s",
				msgID, channelID, userID, GetModuleNameById(module),
			)
			return true
		}
	}

	// denied role
	for _, userRoleID := range userRoles {
		if GetDeniedForRole(channel.GuildID, userRoleID)&module == module ||
			GetDeniedForRole(channel.GuildID, userRoleID)&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
			cache.GetLogger().WithField("module", "modulepermissions").Infof(
				"denied command by role message #%s channel #%s user #%s module %s",
				msgID, channelID, userID, GetModuleNameById(module),
			)
			return false
		}
	}

	// allowed channel
	if GetAllowedForChannel(channel.GuildID, channelID)&module == module ||
		GetAllowedForChannel(channel.GuildID, channelID)&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
		cache.GetLogger().WithField("module", "modulepermissions").Infof(
			"allowed command by channel message #%s channel #%s user #%s module %s",
			msgID, channelID, userID, GetModuleNameById(module),
		)
		return true
	}

	// denied channel
	if GetDeniedForChannel(channel.GuildID, channelID)&module == module ||
		GetDeniedForChannel(channel.GuildID, channelID)&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
		cache.GetLogger().WithField("module", "modulepermissions").Infof(
			"denied command by channel message #%s channel #%s user #%s module %s",
			msgID, channelID, userID, GetModuleNameById(module),
		)
		return false
	}

	// allowed parent channel (if in sync)
	if checkParent {
		if GetAllowedForChannel(channel.GuildID, channel.ParentID)&module == module ||
			GetAllowedForChannel(channel.GuildID, channel.ParentID)&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
			cache.GetLogger().WithField("module", "modulepermissions").Infof(
				"allowed command by parent channel message #%s channel #%s user #%s module %s",
				msgID, channelID, userID, GetModuleNameById(module),
			)
			return true
		}
	}

	// denied parent channel (if in sync)
	if checkParent {
		if GetDeniedForChannel(channel.GuildID, channel.ParentID)&module == module ||
			GetDeniedForChannel(channel.GuildID, channel.ParentID)&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
			cache.GetLogger().WithField("module", "modulepermissions").Infof(
				"denied command by parent channel message #%s channel #%s user #%s module %s",
				msgID, channelID, userID, GetModuleNameById(module),
			)
			return false
		}
	}

	return true
}

func GetModuleNameById(id models.ModulePermissionsModule) (name string) {
	for _, module := range Modules {
		if module.Permission == id {
			if len(module.Names) > 0 {
				return module.Names[0]
			}
			return "unnamed"
		}
	}
	if (ModulePermAll | ModulePermAllPlaceholder) == id {
		return "all modules"
	}
	return "not found"
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
		if GetDeniedForRole(guildID, everyoneRoleID)&module.Permission == module.Permission {
			// is denied for everyone
			for _, role := range guild.Roles {
				if GetAllowedForRole(guildID, role.ID)&module.Permission == module.Permission {
					continue NextModule
				}
			}

			disabledModules = append(disabledModules, module.Permission)
		}
	}

	return disabledModules
}

func SetAllowedForChannel(guildID, channelID string, newPermissions models.ModulePermissionsModule) (err error) {
	entry := findModulePermissionsEntry(guildID, "channel", channelID)
	entry.Allowed = newPermissions
	err = setModulePermissionsEntry(entry)
	return err
}

func SetAllowedForRole(guildID, roleID string, newPermissions models.ModulePermissionsModule) (err error) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	entry.Allowed = newPermissions
	err = setModulePermissionsEntry(entry)
	return err
}

func SetDeniedForChannel(guildID, channelID string, newPermissions models.ModulePermissionsModule) (err error) {
	entry := findModulePermissionsEntry(guildID, "channel", channelID)
	entry.Denied = newPermissions
	err = setModulePermissionsEntry(entry)
	return err
}

func SetDeniedForRole(guildID, roleID string, newPermissions models.ModulePermissionsModule) (err error) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	entry.Denied = newPermissions
	err = setModulePermissionsEntry(entry)
	return err
}

func GetAllowedForChannel(guildID, channelID string) (permissions models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "channel", channelID)
	if entry.Allowed >= 0 {
		if entry.Allowed&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
			return ModulePermAll | ModulePermAllPlaceholder
		}
		return entry.Allowed
	}
	return 0
}

func GetAllowedForRole(guildID, roleID string) (permissions models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	if entry.Allowed >= 0 {
		if entry.Allowed&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
			return ModulePermAll | ModulePermAllPlaceholder
		}
		return entry.Allowed
	}
	return 0
}

func GetDeniedForChannel(guildID, channelID string) (permissions models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "channel", channelID)
	if entry.Denied >= 0 {
		if entry.Denied&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
			return ModulePermAll | ModulePermAllPlaceholder
		}
		return entry.Denied
	}
	return 0
}

func GetDeniedForRole(guildID, roleID string) (permissions models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	if entry.Denied >= 0 {
		if entry.Denied&ModulePermAllPlaceholder == ModulePermAllPlaceholder {
			return ModulePermAll | ModulePermAllPlaceholder
		}
		return entry.Denied
	}
	return 0
}

func findModulePermissionsEntry(guildID, permType, targetID string) (entry models.ModulePermission) {
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

	listCursor, err := gorethink.Table(models.ModulePermissionsTable).Filter(
		gorethink.Row.Field("guild_id").Eq(guildID),
	).Filter(
		gorethink.Row.Field("type").Eq(permType),
	).Filter(
		gorethink.Row.Field("target_id").Eq(targetID),
	).Run(GetDB())
	if err != nil {
		entry = models.GetDefaultModulePermission()
		entry.GuildID = guildID
		entry.Type = permType
		entry.TargetID = targetID
		return entry
	}
	defer listCursor.Close()
	err = listCursor.One(&entry)
	if err != nil {
		entry = models.GetDefaultModulePermission()
		entry.GuildID = guildID
		entry.Type = permType
		entry.TargetID = targetID
		return entry
	}

	return entry
}

func GetModulePermissionEntries(guildID string) (entries []models.ModulePermission) {
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

	listCursor, err := gorethink.Table(models.ModulePermissionsTable).Filter(
		gorethink.Row.Field("guild_id").Eq(guildID),
	).Run(GetDB())
	if err != nil {
		return entries
	}
	defer listCursor.Close()
	err = listCursor.All(&entries)
	if err != nil {
		return entries
	}

	return entries
}

func setModulePermissionsEntry(entry models.ModulePermission) (err error) {
	if entry.ID != "" {
		_, err = gorethink.Table(models.ModulePermissionsTable).Update(entry).Run(GetDB())
		go func() {
			defer Recover()

			err := RefreshModulePermissionsCache()
			Relax(err)
		}()
		return err
	}
	_, err = gorethink.Table(models.ModulePermissionsTable).Insert(entry).RunWrite(GetDB())
	go func() {
		defer Recover()

		err := RefreshModulePermissionsCache()
		Relax(err)
	}()
	return err
}
