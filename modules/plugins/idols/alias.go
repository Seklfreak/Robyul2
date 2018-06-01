package idols

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
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

// addGroupAlias will add an alias for a group
func addGroupAlias(msg *discordgo.Message, content string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// validate arguments
	commandArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	if len(commandArgs) != 4 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	targetGroup := commandArgs[2]
	newAliasName := commandArgs[3]

	// check that the group we're adding the alias too actually exists
	if exists, realGroupName := GetMatchingGroup(targetGroup); exists == false {
		helpers.SendMessage(msg.ChannelID, "Could not add alias for that group because the group does not exist.")
		return
	} else {
		targetGroup = realGroupName
	}

	// make sure the alias doesn't match an existing group already
	if exists, matchinGroup := GetMatchingGroup(newAliasName); exists {
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("The alias you are trying to add already exists for the group **%s**", matchinGroup))
		return
	}

	// check if the alias already exists
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	newAliasReg := strings.ToLower(reg.ReplaceAllString(newAliasName, ""))
	for curGroup, aliases := range getGroupAliases() {
		for _, alias := range aliases {
			curAlias := strings.ToLower(reg.ReplaceAllString(alias, ""))

			if curAlias == newAliasReg {
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
func deleteGroupAlias(msg *discordgo.Message, content string) {
	cache.GetSession().ChannelTyping(msg.ChannelID)

	// validate arguments
	commandArgs, err := helpers.ToArgv(content)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}
	if len(commandArgs) != 3 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

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

// listGroupAliases will display the current aliases in a embed message
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
		if exists, realGroupName := GetMatchingGroup(groupName); exists {

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
