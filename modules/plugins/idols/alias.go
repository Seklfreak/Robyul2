package idols

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
)

const (
	GROUP_ALIAS_KEY = "groupAliases"
)

// maps real group name => aliases for group
var groupAliasesMap map[string][]string
var groupAliasMutex sync.RWMutex

// initAliases will load the aliases object from the database
func initAliases() {
	groupAliasesMap = make(map[string][]string)

	groupAliasMutex.Lock()
	getModuleCache(GROUP_ALIAS_KEY, &groupAliasesMap)
	groupAliasMutex.Unlock()
}

// getGroupAliases gets the current group aliases
func getGroupAliases() map[string][]string {
	groupAliasMutex.RLock()
	defer groupAliasMutex.RUnlock()
	return groupAliasesMap
}

// addGroupAlias will add an alias for a group or idol depending on the amount of arguments
func addAlias(msg *discordgo.Message, content string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// validate arguments
	commandArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	// IDOL ALIAS
	if len(commandArgs) == 5 {
		addIdolAlias(msg, commandArgs[2], commandArgs[3], commandArgs[4])
		return
	}

	// GROUP ALIAS
	if len(commandArgs) == 4 {
		addGroupAlias(msg, commandArgs[2], commandArgs[3])
		return
	}

	helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
}

// addIdolAlias will add an alias for a idol
func addIdolAlias(msg *discordgo.Message, targetGroup string, targetName string, newAliasName string) {

	// check that the idol we're adding the alias too actually exists
	var targetIdol *Idol

	if _, _, targetIdol = GetMatchingIdolAndGroup(targetGroup, targetName, true); targetIdol == nil {
		helpers.SendMessage(msg.ChannelID, "Could not add alias for that idol because the idol could not be found.")
		return
	}

	// make map of group => []idol names and aliases
	groupIdolMap := make(map[string][]string)
	for _, idol := range GetAllIdols() {
		if idol.GroupName == targetIdol.GroupName {
			groupIdolMap[idol.GroupName] = append(groupIdolMap[idol.GroupName], idol.Name)
			groupIdolMap[idol.GroupName] = append(groupIdolMap[idol.GroupName], idol.NameAliases...)
		}
	}

	// confirm new alias doesn't match alias or name within a group
	for _, currentNamesOrAliases := range groupIdolMap {
		for _, currentName := range currentNamesOrAliases {
			if alphaNumericCompare(currentName, newAliasName) {
				helpers.SendMessage(msg.ChannelID, "That alias already exists for an idol in the group.")
				return
			}
		}
	}

	// add alias in memory
	allIdolsMutex.Lock()
	targetIdol.NameAliases = append(targetIdol.NameAliases, newAliasName)
	allIdolsMutex.Unlock()

	// update cache
	if len(GetAllIdols()) > 0 {
		setModuleCache(ALL_IDOLS_CACHE_KEY, GetAllIdols(), time.Hour*24*7)
	}

	// add alias in mongo
	var mongoIdol models.IdolEntry
	err := helpers.MdbOne(helpers.MdbCollection(models.IdolTable).Find(bson.M{"groupname": targetIdol.GroupName, "name": targetIdol.Name}), &mongoIdol)
	helpers.Relax(err)

	mongoIdol.NameAliases = append(mongoIdol.NameAliases, newAliasName)

	// save target idol with new images
	err = helpers.MDbUpsertID(models.IdolTable, mongoIdol.ID, mongoIdol)
	helpers.Relax(err)

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("The alias *%s* has been added for %s %s", newAliasName, targetIdol.GroupName, targetIdol.Name))
}

// addGroupAlias will add an alias for a group
func addGroupAlias(msg *discordgo.Message, targetGroup string, newAliasName string) {

	// check that the group we're adding the alias too actually exists
	if exists, realGroupName := GetMatchingGroup(targetGroup, true); exists == false {
		helpers.SendMessage(msg.ChannelID, "Could not add alias for that group because the group does not exist.")
		return
	} else {
		targetGroup = realGroupName
	}

	// make sure the alias doesn't match an existing group already
	if exists, matchinGroup := GetMatchingGroup(newAliasName, false); exists {
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("The alias you are trying to add already exists for the group **%s**", matchinGroup))
		return
	}

	// check if the alias already exists
	for curGroup, aliases := range getGroupAliases() {
		for _, alias := range aliases {
			if alphaNumericCompare(newAliasName, alias) {
				helpers.SendMessage(msg.ChannelID, fmt.Sprintf("This group alias already exists for the group **%s**", curGroup))
				return
			}
		}
	}

	// add the alias to the alias map
	groupAliasMutex.Lock()
	groupAliasesMap[targetGroup] = append(groupAliasesMap[targetGroup], newAliasName)
	groupAliasMutex.Unlock()

	// save to redis
	setModuleCache(GROUP_ALIAS_KEY, getGroupAliases(), 0)

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("The alias *%s* has been added for the group **%s**", newAliasName, targetGroup))
}

// deleteGroupAlias will delete the alias if it is found
func deleteIdolAlias(msg *discordgo.Message, commandArgs []string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	targetGroup := commandArgs[2]
	targetName := commandArgs[3]
	aliasToDelete := commandArgs[4]

	var targetIdol *Idol
	if _, _, targetIdol = GetMatchingIdolAndGroup(targetGroup, targetName, false); targetIdol == nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-matching-idol"))
		return
	}

	aliasFound := false
	for index, alias := range targetIdol.NameAliases {
		if alphaNumericCompare(alias, aliasToDelete) {
			aliasToDelete = alias
			aliasFound = true
			allIdolsMutex.Lock()
			targetIdol.NameAliases = append(targetIdol.NameAliases[:index], targetIdol.NameAliases[index+1:]...)
			allIdolsMutex.Unlock()
			break
		}
	}

	if aliasFound == false {
		helpers.SendMessage(msg.ChannelID, "That alias was not found for the given idol.")
		return
	}

	// update cache
	if len(GetAllIdols()) > 0 {
		setModuleCache(ALL_IDOLS_CACHE_KEY, GetAllIdols(), time.Hour*24*7)
	}

	var mongoIdol models.IdolEntry
	err := helpers.MdbOne(helpers.MdbCollection(models.IdolTable).Find(bson.M{"groupname": targetIdol.GroupName, "name": targetIdol.Name}), &mongoIdol)
	helpers.Relax(err)

	for index, alias := range mongoIdol.NameAliases {
		if alphaNumericCompare(alias, aliasToDelete) {
			mongoIdol.NameAliases = append(mongoIdol.NameAliases[:index], mongoIdol.NameAliases[index+1:]...)
			break
		}
	}
	err = helpers.MDbUpsertID(models.IdolTable, mongoIdol.ID, mongoIdol)
	helpers.Relax(err)

	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Deleted the alias *%s* from %s %s", aliasToDelete, targetIdol.GroupName, targetIdol.Name))
}

// deleteGroupAlias will delete the alias if it is found
func deleteGroupAlias(msg *discordgo.Message, commandArgs []string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	aliasToDelete := commandArgs[2]

	// find and delete alias if one exists
	aliasDeleted := false
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	regToDelete := strings.ToLower(reg.ReplaceAllString(aliasToDelete, ""))
	groupAliasMutex.Lock()
GroupAliasLoop:
	for curGroup, aliases := range groupAliasesMap {
		for i, alias := range aliases {
			curAlias := strings.ToLower(reg.ReplaceAllString(alias, ""))

			if curAlias == regToDelete {

				// if the alias is the last one for the group, remove the group from the alias map
				if len(aliases) == 1 {
					delete(groupAliasesMap, curGroup)
				} else {
					aliases = append(aliases[:i], aliases[i+1:]...)
					groupAliasesMap[curGroup] = aliases
				}

				aliasDeleted = true
				helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Deleted the alias *%s* from the group **%s**", alias, curGroup))
				break GroupAliasLoop
			}
		}
	}
	groupAliasMutex.Unlock()

	// if no alias was deleted, send a message
	if aliasDeleted {
		// save to redis
		setModuleCache(GROUP_ALIAS_KEY, getGroupAliases(), 0)
	} else {
		helpers.SendMessage(msg.ChannelID, "Alias not found, no alias was deleted")
	}
}

// listAliases will list group aliases or idol name aliases for a group or idol
func listAliases(msg *discordgo.Message, content string) {
	contentArgs, err := helpers.ToArgv(content)
	helpers.Relax(err)

	// if enough args were passed, attempt to list aliases for idols
	switch len(contentArgs) {
	case 2:
		listGroupAliases(msg)
		break
	case 3:
		listNameAliasesByGroup(msg, contentArgs[2])
		break
	case 4:
		listNameAliases(msg, contentArgs[2], contentArgs[3])
		break
	default:
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))

	}
}

// listNameAliases lists aliases for a idol
func listNameAliases(msg *discordgo.Message, targetGroup string, targetName string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	var targetIdol *Idol
	if _, _, targetIdol = GetMatchingIdolAndGroup(targetGroup, targetName, true); targetIdol == nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-matching-idol"))
		return
	}

	// make sure there are aliases to display
	if len(targetIdol.NameAliases) == 0 {
		helpers.SendMessage(msg.ChannelID, "No aliases have been set for the given idol.")
		return
	}

	// set up base embed
	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("Current aliases for %s %s", targetIdol.GroupName, targetIdol.Name),
			IconURL: msg.Author.AvatarURL("512"),
		},
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Name Aliases",
		Value:  strings.Join(targetIdol.NameAliases, ", "),
		Inline: true,
	})

	helpers.SendEmbed(msg.ChannelID, embed)
}

// listNameAliasesByGroup lists aliases for a group
func listNameAliasesByGroup(msg *discordgo.Message, targetGroup string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	var realGroupName string
	if _, realGroupName = GetMatchingGroup(targetGroup, true); realGroupName == "" {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.stats.no-matching-group"))
		return
	}

	// set up base embed
	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("Current aliases for %s", realGroupName),
			IconURL: msg.Author.AvatarURL("512"),
		},
	}

	// add field for group alias for the given group
	var aliasesForThisGroup []string
	for group, aliases := range getGroupAliases() {
		if alphaNumericCompare(realGroupName, group) {
			aliasesForThisGroup = aliases
		}
	}

	if len(aliasesForThisGroup) > 0 {

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Group Aliases",
			Value:  strings.Join(aliasesForThisGroup, ", "),
			Inline: false,
		})
	}

	// add fields for name aliases for all idols
	for _, idol := range GetActiveIdols() {
		if realGroupName == idol.GroupName && len(idol.NameAliases) > 0 {

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   idol.Name,
				Value:  strings.Join(idol.NameAliases, ", "),
				Inline: false,
			})
		}
	}

	// make sure there are aliases to display
	if len(embed.Fields) == 0 {
		helpers.SendMessage(msg.ChannelID, "No aliases have been set yet for the given group.")
		return
	}

	helpers.SendPagedMessage(msg, embed, 7)
}

// listGroupAliases will display the current group aliases in a embed message
func listGroupAliases(msg *discordgo.Message) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// set up base embed
	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    "Current group aliases",
			IconURL: msg.Author.AvatarURL("512"),
		},
	}

	groupAliases := getGroupAliases()

	// get group names into a slice so they can be sorted
	groups := make([]string, len(groupAliases))
	for group, _ := range groupAliases {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i] < groups[j]
	})

	// get aliases for each group and add them to the embed
	for _, groupName := range groups {
		aliases := groupAliases[groupName]

		// sort aliases
		sort.Slice(aliases, func(i, j int) bool {
			return aliases[i] < aliases[j]
		})

		// get the matching group, the aliases might have been saved before a small change was made to the real group name. i want to account for that
		if exists, realGroupName := GetMatchingGroup(groupName, true); exists {

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   realGroupName,
				Value:  strings.Join(aliases, ", "),
				Inline: false,
			})
		}
	}

	// make sure there are aliases to display
	if len(embed.Fields) == 0 {
		helpers.SendMessage(msg.ChannelID, "No aliases have been set yet.")
		return
	}

	// send paged message with 7 fields per page
	helpers.SendPagedMessage(msg, embed, 7)
}

// getAlisesForGroup gets the aliases for a group if it exists.
//  first return will be false if the group was not found
func getAlisesForGroup(targetGroup string) (bool, []string) {

	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	for aliasGroup, aliases := range getGroupAliases() {
		group := strings.ToLower(reg.ReplaceAllString(aliasGroup, ""))

		if targetGroup == group {
			return true, aliases
		}
	}

	return false, nil
}
