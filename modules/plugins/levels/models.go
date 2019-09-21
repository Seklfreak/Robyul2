package levels

import (
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
)

func getLevelsServerUserOrCreateNewWithoutLogging(guildid string, userid string) (serveruser models.LevelsServerusersEntry, err error) {
	err = helpers.MdbOneWithoutLogging(
		helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"userid": userid, "guildid": guildid}),
		&serveruser,
	)

	if err != nil && helpers.IsMdbNotFound(err) {
		serveruser.UserID = userid
		serveruser.GuildID = guildid
		newid, err := helpers.MDbInsertWithoutLogging(models.LevelsServerusersTable, serveruser)
		serveruser.ID = newid
		return serveruser, err
	}

	return serveruser, err
}

func getLevelsRoles(guildID string, currentLevel int) (apply []*discordgo.Role, remove []*discordgo.Role) {
	apply = make([]*discordgo.Role, 0)
	remove = make([]*discordgo.Role, 0)

	var entryBucket []models.LevelsRoleEntry
	err := helpers.MDbIterWithoutLogging(helpers.MdbCollection(models.LevelsRolesTable).Find(bson.M{"guildid": guildID})).All(&entryBucket)
	if err != nil {
		helpers.RelaxLog(err)
		return
	}

	if len(entryBucket) <= 0 {
		return
	}

	for _, entry := range entryBucket {
		role, err := cache.GetSession().SessionForGuildS(guildID).State.Role(guildID, entry.RoleID)
		if err != nil {
			continue
		}

		if currentLevel >= entry.StartLevel && (entry.LastLevel < 0 || currentLevel <= entry.LastLevel) {
			apply = append(apply, role)
		} else {
			remove = append(remove, role)
		}
	}

	return
}

func getLevelsRolesUserOverwrites(guildID string, userID string) (overwrites []models.LevelsRoleOverwriteEntry) {
	err := helpers.MDbIter(helpers.MdbCollection(models.LevelsRoleOverwritesTable).Find(bson.M{"userid": userID, "guildid": guildID})).All(&overwrites)
	if err != nil {
		helpers.RelaxLog(err)
		return make([]models.LevelsRoleOverwriteEntry, 0)
	}
	return overwrites
}

func getLevelForUser(userID string, guildID string) int {
	var levelsServersUser []models.LevelsServerusersEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"userid": userID})).All(&levelsServersUser)
	helpers.Relax(err)

	if levelsServersUser == nil {
		return 0
	} else if err != nil {
		helpers.Relax(err)
	}

	if guildID == "global" {
		totalExp := int64(0)
		for _, levelsServerUser := range levelsServersUser {
			totalExp += levelsServerUser.Exp
		}
		return GetLevelFromExp(totalExp)
	} else {
		for _, levelsServerUser := range levelsServersUser {
			if levelsServerUser.GuildID == guildID {
				return GetLevelFromExp(levelsServerUser.Exp)
			}
		}
	}

	return 0
}

func getBadge(category string, name string, guildID string) models.ProfileBadgeEntry {
	var entryBucket []models.ProfileBadgeEntry
	var emptyBadge models.ProfileBadgeEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.ProfileBadgesTable).Find(bson.M{"category": strings.ToLower(category)})).All(&entryBucket)
	if err != nil {
		panic(err)
	}

	if entryBucket == nil || len(entryBucket) <= 0 {
		return emptyBadge
	} else if err != nil {
		helpers.Relax(err)
	}

	for _, badge := range entryBucket {
		if strings.ToLower(badge.Name) == strings.ToLower(name) && (badge.GuildID == guildID || badge.GuildID == "global") {
			return badge
		}
	}

	return emptyBadge
}

func getCategoryBadges(category string, guildID string) (badges []models.ProfileBadgeEntry) {
	err := helpers.MDbIter(helpers.MdbCollection(models.ProfileBadgesTable).Find(bson.M{"category": strings.ToLower(category), "guildid": guildID})).All(&badges)
	if err != nil {
		panic(err)
	}
	return
}

func getServerOnlyBadges(guildID string) []models.ProfileBadgeEntry {
	var entryBucket []models.ProfileBadgeEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.ProfileBadgesTable).Find(bson.M{"guildid": guildID})).All(&entryBucket)
	if err != nil {
		panic(err)
	}
	return entryBucket
}

func getServerBadges(guildID string) []models.ProfileBadgeEntry {
	entryBucket := getServerOnlyBadges(guildID)

	var globalEntryBucket []models.ProfileBadgeEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.ProfileBadgesTable).Find(bson.M{"guildid": "global"})).All(&globalEntryBucket)
	if err != nil {
		panic(err)
	}
	for _, globalEntry := range globalEntryBucket {
		if globalEntry.GuildID == guildID || globalEntry.GuildID == "global" {
			entryBucket = append(entryBucket, globalEntry)
		}
	}

	return entryBucket
}

func getBadgeByID(badgeID string) (badge models.ProfileBadgeEntry) {
	err := helpers.MdbOne(
		helpers.MdbCollection(models.ProfileBadgesTable).Find(bson.M{"_id": helpers.HumanToMdbId(badgeID)}),
		&badge,
	)
	if err != nil {
		if helpers.IsMdbNotFound(err) {
			err := helpers.MdbOne(
				helpers.MdbCollection(models.ProfileBadgesTable).Find(bson.M{"oldid": badgeID}),
				&badge,
			)
			if err != nil {
				if helpers.IsMdbNotFound(err) {
					return
				} else {
					panic(err)
				}
			}
		} else {
			panic(err)
		}
	}

	return
}

func getBadgesAvailable(user *discordgo.User, sourceServerID string) []models.ProfileBadgeEntry {
	guildsToCheck := make([]string, 0)
	guildsToCheck = append(guildsToCheck, "global")

	for _, shard := range cache.GetSession().Sessions {
		for _, guild := range shard.State.Guilds {
			if helpers.GetIsInGuild(guild.ID, user.ID) {
				guildsToCheck = append(guildsToCheck, guild.ID)
			}
		}
	}

	sourceServerAlreadyIn := false
	for _, guild := range guildsToCheck {
		if guild == sourceServerID {
			sourceServerAlreadyIn = true
		}
	}
	if sourceServerAlreadyIn == false {
		guildsToCheck = append(guildsToCheck, sourceServerID)
	}

	var allBadges []models.ProfileBadgeEntry
	for _, guildToCheck := range guildsToCheck {
		entryBucket := getServerBadges(guildToCheck)
		for _, entryBadge := range entryBucket {
			allBadges = append(allBadges, entryBadge)
		}
	}

	levelCache := make(map[string]int, 0)

	var availableBadges []models.ProfileBadgeEntry
	for _, foundBadge := range allBadges {
		isAllowed := false

		// Level Check
		if foundBadge.LevelRequirement < 0 { // Available for no one?
			isAllowed = false
		} else if foundBadge.LevelRequirement == 0 { // Available for everyone?
			isAllowed = true
		} else if foundBadge.LevelRequirement > 0 { // Meets min level?
			if _, ok := levelCache[foundBadge.GuildID]; ok {
				if foundBadge.LevelRequirement <= levelCache[foundBadge.GuildID] {
					isAllowed = true
				} else {
					isAllowed = false
				}
			} else {
				levelCache[foundBadge.GuildID] = getLevelForUser(user.ID, foundBadge.GuildID)
				if foundBadge.LevelRequirement <= levelCache[foundBadge.GuildID] {
					isAllowed = true
				} else {
					isAllowed = false
				}
			}
		}
		// Role Check
		if foundBadge.RoleRequirement != "" { // is there a role requirement?
			isAllowed = false
			member, err := helpers.GetGuildMemberWithoutApi(foundBadge.GuildID, user.ID)
			if err == nil {
				for _, memberRole := range member.Roles { // check if user got role
					if memberRole == foundBadge.RoleRequirement {
						isAllowed = true
					}
				}
			}
		}

		// User is in allowed user list?
		for _, allowedUserID := range foundBadge.AllowedUserIDs {
			if allowedUserID == user.ID {
				isAllowed = true
			}
		}

		// User is in denied user list?
		for _, deniedUserID := range foundBadge.DeniedUserIDs {
			if deniedUserID == user.ID {
				isAllowed = false
			}
		}

		if isAllowed == true {
			availableBadges = append(availableBadges, foundBadge)
		}
	}

	return availableBadges
}

func getBadgesAvailableQuick(user *discordgo.User, activeBadgeIDs []string) []models.ProfileBadgeEntry {
	activeBadges := make([]models.ProfileBadgeEntry, 0)
	for _, activeBadgeID := range activeBadgeIDs {
		badge := getBadgeByID(activeBadgeID)
		if badge.ID != "" {
			if badge.GuildID == "global" {
				activeBadges = append(activeBadges, badge)
			} else {
				if helpers.GetIsInGuild(badge.GuildID, user.ID) {
					activeBadges = append(activeBadges, badge)
				}
			}
		}
	}

	var availableBadges []models.ProfileBadgeEntry
	for _, foundBadge := range activeBadges {
		isAllowed := true

		// User is in allowed user list?
		for _, allowedUserID := range foundBadge.AllowedUserIDs {
			if allowedUserID == user.ID {
				isAllowed = true
			}
		}

		// User is in denied user list?
		for _, deniedUserID := range foundBadge.DeniedUserIDs {
			if deniedUserID == user.ID {
				isAllowed = false
			}
		}

		if isAllowed == true {
			availableBadges = append(availableBadges, foundBadge)
		}
	}

	return availableBadges
}

func getBadgeUrl(badge models.ProfileBadgeEntry) (link string) {
	if badge.URL != "" {
		return badge.URL
	}
	link, _ = helpers.GetFileLink(badge.ObjectName)
	return link
}

func (l *Levels) ProfileBackgroundSearch(searchText string) (entryBucket []models.ProfileBackgroundEntry) {
	err := helpers.MDbIter(helpers.MdbCollection(models.ProfileBackgroundsTable).Find(bson.M{"name": bson.M{"$regex": bson.RegEx{Pattern: `.*` + searchText + `.*`, Options: "i"}}}).Sort("name")).All(&entryBucket)
	if err != nil {
		panic(err)
	}
	return entryBucket
}

func (l *Levels) GetProfileBackgroundUrl(userdata models.ProfileUserdataEntry) (link string) {
	if userdata.BackgroundObjectName != "" {
		link, err := helpers.GetFileLink(userdata.BackgroundObjectName)
		if err == nil && link != "" {
			return link
		}
		helpers.RelaxLog(err)
	}

	if userdata.Background != "" {
		if strings.HasPrefix(userdata.Background, "http://") ||
			strings.HasPrefix(userdata.Background, "https://") {
			return userdata.Background
		}
		link = l.GetProfileBackgroundUrlByName(userdata.Background)
		if link != "" {
			return link
		}
	}

	return "http://i.imgur.com/I9b74U9.jpg"
}

func (l *Levels) GetProfileBackgroundUrlByName(backgroundName string) string {
	if backgroundName == "" {
		return ""
	}

	var entryBucket models.ProfileBackgroundEntry
	err := helpers.MdbOne(
		helpers.MdbCollection(models.ProfileBackgroundsTable).Find(bson.M{"name": strings.ToLower(backgroundName)}),
		&entryBucket,
	)
	if err != nil && !helpers.IsMdbNotFound(err) {
		panic(err)
	}

	if entryBucket.URL != "" {
		return entryBucket.URL
	}

	link, err := helpers.GetFileLink(entryBucket.ObjectName)
	if err == nil && link != "" {
		return link
	}

	return ""
}
