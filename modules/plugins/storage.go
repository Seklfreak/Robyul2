package plugins

import (
	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
	"github.com/sirupsen/logrus"
)

type storageAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next storageAction)

type Storage struct{}

func (m *Storage) Commands() []string {
	return []string{
		"storage",
	}
}

func (m *Storage) Init(session *discordgo.Session) {
}

func (m *Storage) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermStats) {
		return
	}

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := m.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (m *Storage) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) storageAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	return m.actionStatus
}

// [p]storage
func (m *Storage) actionStatus(args []string, in *discordgo.Message, out **discordgo.MessageSend) storageAction {
	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	guild, err := helpers.GetGuild(channel.GuildID)
	helpers.Relax(err)

	targetUser, err := helpers.GetUser(in.Author.ID)
	helpers.Relax(err)

	if len(args) >= 1 && helpers.IsRobyulMod(in.Author.ID) {
		targetUser, err = helpers.GetUserFromMention(args[0])
		helpers.Relax(err)
	}

	// request all user files
	var entryBucket []models.StorageEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.StorageTable).Find(bson.M{"userid": targetUser.ID})).All(&entryBucket)

	// request guild and global stats
	totalGuildFiles, err := helpers.MdbCount(models.StorageTable, bson.M{"guildid": guild.ID})
	helpers.Relax(err)
	totalFiles, err := helpers.MdbCount(models.StorageTable, nil)
	helpers.Relax(err)

	// don't send stats if no files found
	if entryBucket == nil || len(entryBucket) <= 0 {
		*out = m.newMsg("plugins.storage.no-stats-for-user")
		return m.actionFinish
	}

	// calculate stats
	totalUserFiles := len(entryBucket)
	var totalUserGuildFiles, totalUserStorage, totalUserTraffic, totalUserGuildStorage, totalUserGuildTraffic uint64
	totalUserBySource := make(map[string]uint64, 0)
	totalUserGuildBySource := make(map[string]uint64, 0)
	for _, storageEntry := range entryBucket {
		if storageEntry.GuildID == guild.ID {
			// calculate guild stats
			totalUserGuildStorage += uint64(storageEntry.Filesize)
			totalUserGuildTraffic += uint64(storageEntry.Filesize * storageEntry.RetrievedCount)
			if _, ok := totalUserGuildBySource[storageEntry.Source]; ok {
				totalUserGuildBySource[storageEntry.Source] += uint64(storageEntry.Filesize)
			} else {
				totalUserGuildBySource[storageEntry.Source] = uint64(storageEntry.Filesize)
			}
			totalUserGuildFiles++
		}
		// calculate total stats
		totalUserStorage += uint64(storageEntry.Filesize)
		totalUserTraffic += uint64(storageEntry.Filesize * storageEntry.RetrievedCount)
		if _, ok := totalUserBySource[storageEntry.Source]; ok {
			totalUserBySource[storageEntry.Source] += uint64(storageEntry.Filesize)
		} else {
			totalUserBySource[storageEntry.Source] = uint64(storageEntry.Filesize)
		}
	}

	// create guild stats text
	var totalUserGuildBySourceText string
	for sourceName, sourceStorage := range totalUserGuildBySource {
		percentage := float64(sourceStorage) / (float64(totalUserGuildStorage) / float64(100))
		if totalUserGuildBySourceText != "" {
			totalUserGuildBySourceText += ", "
		}
		totalUserGuildBySourceText += fmt.Sprintf("%s: %.1f %%", strings.Title(sourceName), percentage)
	}
	if totalUserGuildBySourceText != "" {
		totalUserGuildBySourceText = "\n" + totalUserGuildBySourceText
	}
	// create total stats text
	var totalUserBySourceText string
	for sourceName, sourceStorage := range totalUserBySource {
		percentage := float64(sourceStorage) / (float64(totalUserStorage) / float64(100))
		if totalUserBySourceText != "" {
			totalUserBySourceText += ", "
		}
		totalUserBySourceText += fmt.Sprintf("%s: %.1f %%", strings.Title(sourceName), percentage)
	}
	if totalUserBySourceText != "" {
		totalUserBySourceText = "\n" + totalUserBySourceText
	}

	// create display embed
	embed := &discordgo.MessageEmbed{
		Color: 0xFADED,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("Storage Stats for %s#%s", targetUser.Username, targetUser.Discriminator),
			IconURL: targetUser.AvatarURL("64"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "On " + guild.Name,
				Value: "**Storage:** " + humanize.Bytes(totalUserGuildStorage) +
					fmt.Sprintf(" (%d files)", totalUserGuildFiles) +
					totalUserGuildBySourceText +
					"\n**Traffic:** " + humanize.Bytes(totalUserGuildTraffic),
			},
			{
				Name: "On all Robyul Servers",
				Value: "**Storage:** " + humanize.Bytes(totalUserStorage) +
					fmt.Sprintf(" (%d files)", totalUserFiles) +
					totalUserBySourceText +
					"\n**Traffic:** " + humanize.Bytes(totalUserTraffic),
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%d Files in total on %s | %d Files in total on all Robyul Servers",
				totalGuildFiles, guild.Name, totalFiles),
			IconURL: cache.GetSession().State.User.AvatarURL("64"),
		},
	}

	*out = &discordgo.MessageSend{Embed: embed}
	return m.actionFinish
}

func (m *Storage) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) storageAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (m *Storage) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (m *Storage) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (m *Storage) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "storage")
}
