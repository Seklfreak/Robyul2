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
	ModulePermStats          models.ModulePermissionsModule = 1 << iota // stats.go, uptime.go x
	ModulePermTranslator                                                // translator.go x
	ModulePermUrban                                                     // urbandict.go x
	ModulePermWeather                                                   // weather.go x
	ModulePermVLive                                                     // vlive.go x
	ModulePermInstagram                                                 // instagram.go x
	ModulePermFacebook                                                  // facebook.go x
	ModulePermWolframAlpha                                              // wolframalpha.go x
	ModulePermLastFm                                                    // lastfm.go x
	ModulePermTwitch                                                    // twitch.go x
	ModulePermCharts                                                    // charts.go x
	ModulePermChoice                                                    // choice.go x
	ModulePermOsu                                                       // osu.go x
	ModulePermReminders                                                 // reminders.go x
	ModulePermGfycat                                                    // gfycat.go x
	ModulePermRandomPictures                                            // randompictures.go x
	ModulePermYouTube                                                   // youtube/ x
	ModulePermSpoiler                                                   // spoiler.go x
	ModulePermRandomCat                                                 // random_cat.go x
	ModulePermRPS                                                       // rps.go x
	ModulePermDig                                                       // dig.go x
	ModulePermStreamable                                                // streamable.go x
	ModulePermLyrics                                                    // lyrics.go x
	ModulePermNames                                                     // names.go x
	ModulePermReddit                                                    // reddit.go x
	ModulePermColor                                                     // color.go x
	ModulePermDog                                                       // dog.go x
	ModulePermGoogle                                                    // google/ x
	ModulePermWhois                                                     // whois.go x
	ModulePermIsup                                                      // isup.go
	ModulePermLevels                                                    // levels.go
	ModulePermCustomCommands                                            // customcommands.go
	ModulePermReactionPolls                                             // reactionpolls.go
	ModulePermTwitter                                                   // twitter.go
	ModulePermStarboard                                                 // starboard.go
	//ModulePerm8ball                                                     // 8ball.go

	ModulePermAll = ModulePermStats | ModulePermTranslator | ModulePermUrban | ModulePermWeather | ModulePermVLive |
		ModulePermInstagram | ModulePermFacebook | ModulePermWolframAlpha | ModulePermLastFm | ModulePermTwitter |
		ModulePermTwitch | ModulePermCharts | ModulePermChoice | ModulePermOsu | ModulePermReminders | ModulePermGfycat |
		ModulePermRandomPictures | ModulePermYouTube | ModulePermSpoiler | ModulePermRandomCat | ModulePermRPS |
		ModulePermDig | ModulePermStreamable | ModulePermLyrics | ModulePermNames | ModulePermReddit | ModulePermColor |
		ModulePermDog | ModulePermGoogle | ModulePermWhois | ModulePermIsup | ModulePermLevels | ModulePermCustomCommands |
		ModulePermReactionPolls | ModulePermTwitter | ModulePermStarboard
	// | ModulePerm8ball
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
		//{Names: []string{"8ball"}, Permission: ModulePerm8ball},
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
		cache.GetLogger().WithField("module", "modulepermissions").Error(
			"failed to get channel for ModuleIsAllowedSilent:", err.Error(),
		)
		return true
	}

	user, err := GetGuildMemberWithoutApi(channel.GuildID, userID)
	if err != nil {
		cache.GetLogger().WithField("module", "modulepermissions").Error(
			"failed to get user for ModuleIsAllowedSilent:", err.Error(),
		)
		return true
	}

	guild, err := GetGuildWithoutApi(channel.GuildID)
	if err != nil {
		cache.GetLogger().WithField("module", "modulepermissions").Error(
			"failed to get guild for ModuleIsAllowedSilent:", err.Error(),
		)
		return true
	}

	var everyoneRoleID string
	for _, role := range guild.Roles {
		if role.Name == "@everyone" {
			everyoneRoleID = role.ID
		}
	}
	if everyoneRoleID != "" {
		user.Roles = append(user.Roles, everyoneRoleID)
	}

	// allowed roles > allowed channels > denied roles > denied channels

	for _, userRoleID := range user.Roles {
		if GetAllowedForRole(channel.GuildID, userRoleID)&module == module {
			cache.GetLogger().WithField("module", "modulepermissions").Infof(
				"allowed command by role message #%s channel #%s user #%d module %s",
				msgID, channelID, userID, GetModuleNameById(module),
			)
			return true
		}
	}

	if GetAllowedForChannel(channel.GuildID, channelID)&module == module {
		cache.GetLogger().WithField("module", "modulepermissions").Infof(
			"allowed command by channel message #%s channel #%s user #%d module %s",
			msgID, channelID, userID, GetModuleNameById(module),
		)
		return true
	}

	for _, userRoleID := range user.Roles {
		if GetDeniedForRole(channel.GuildID, userRoleID)&module == module {
			cache.GetLogger().WithField("module", "modulepermissions").Infof(
				"denied command by role message #%s channel #%s user #%d module %s",
				msgID, channelID, userID, GetModuleNameById(module),
			)
			return false
		}
	}

	if GetDeniedForChannel(channel.GuildID, channelID)&module == module {
		cache.GetLogger().WithField("module", "modulepermissions").Infof(
			"denied command by channel message #%s channel #%s user #%d module %s",
			msgID, channelID, userID, GetModuleNameById(module),
		)
		return false
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
	return "not found"
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
		return entry.Allowed
	}
	return 0
}

func GetAllowedForRole(guildID, roleID string) (permissions models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	if entry.Allowed >= 0 {
		return entry.Allowed
	}
	return 0
}

func GetDeniedForChannel(guildID, channelID string) (permissions models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "channel", channelID)
	if entry.Denied >= 0 {
		return entry.Denied
	}
	return 0
}

func GetDeniedForRole(guildID, roleID string) (permissions models.ModulePermissionsModule) {
	entry := findModulePermissionsEntry(guildID, "role", roleID)
	if entry.Denied >= 0 {
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
