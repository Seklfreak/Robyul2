package helpers

import (
	"encoding/json"
	"time"

	"regexp"

	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
	"github.com/pkg/errors"
)

var isValidVanityName = regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString

func UpdateOrInsertVanityUrl(vanityName, guildID, channelID, userID string) (err error) {
	if !isValidVanityName(vanityName) {
		return errors.New("invalid vanity name")
	}

	vanityEntryByName, _ := GetVanityUrlByVanityName(vanityName)
	if vanityEntryByName.VanityName != "" && vanityEntryByName.GuildID != guildID {
		return errors.New("vanity name already in use")
	}

	vanityEntryByGuildID, _ := GetVanityUrlByGuildID(guildID)
	if vanityEntryByGuildID.VanityName != "" {
		// update vanity invite
		oldVanityInvite := vanityEntryByGuildID

		vanityEntryByGuildID.VanityName = strings.ToLower(vanityName)
		vanityEntryByGuildID.VanityNamePretty = vanityName
		vanityEntryByGuildID.ChannelID = channelID
		vanityEntryByGuildID.SetAt = time.Now()
		vanityEntryByGuildID.SetByUserID = userID

		err = ResetCachedDiscordInviteByVanityInvite(vanityEntryByGuildID)
		RelaxLog(err)

		cache.GetLogger().WithField("module", "helpers/vanityinvite").
			WithField("guildID", vanityEntryByGuildID.GuildID).
			Info("updated the vanity url, new name:", vanityEntryByGuildID.VanityNamePretty, "new channel:", vanityEntryByGuildID.ChannelID)

		err = UpdateVanityUrl(vanityEntryByGuildID)
		if err != nil {
			return err
		}

		go func() {
			logChannelID, _ := GetBotConfigString(models.VanityInviteLogChannelKey)
			if logChannelID != "" {
				err = logVanityInviteChange(logChannelID, vanityEntryByGuildID, oldVanityInvite)
				RelaxLog(err)
			}
		}()

		_, err = EventlogLog(time.Now(), guildID, guildID,
			models.EventlogTargetTypeGuild, userID,
			models.EventlogTypeRobyulVanityInviteUpdate, "",
			[]models.ElasticEventlogChange{
				{
					Key:      "vanityinvite_name",
					OldValue: oldVanityInvite.VanityName,
					NewValue: vanityEntryByGuildID.VanityName,
				},
				{
					Key:      "vanityinvite_channelid",
					OldValue: oldVanityInvite.ChannelID,
					NewValue: vanityEntryByGuildID.ChannelID,
					Type:     models.EventlogTargetTypeChannel,
				},
			},
			nil, false)
		RelaxLog(err)

		return nil
	}

	// create vanity invite
	newVanityInvite := models.VanityInviteEntry{
		GuildID:          guildID,
		VanityName:       strings.ToLower(vanityName),
		VanityNamePretty: vanityName,
		SetByUserID:      userID,
		SetAt:            time.Now(),
		ChannelID:        channelID,
	}

	cache.GetLogger().WithField("module", "helpers/vanityinvite").
		WithField("guildID", vanityEntryByGuildID.GuildID).
		Info("created the vanity url, name:", vanityEntryByGuildID.VanityNamePretty, "channel:", vanityEntryByGuildID.ChannelID)

	_, err = MDbInsert(
		models.VanityInvitesTable,
		newVanityInvite,
	)
	if err != nil {
		return err
	}

	go func() {
		logChannelID, _ := GetBotConfigString(models.VanityInviteLogChannelKey)
		if logChannelID != "" {
			err = logVanityInviteCreation(logChannelID, newVanityInvite)
			RelaxLog(err)
		}
	}()

	_, err = EventlogLog(time.Now(), guildID, guildID,
		models.EventlogTargetTypeGuild, userID,
		models.EventlogTypeRobyulVanityInviteCreate, "",
		nil,
		[]models.ElasticEventlogOption{
			{
				Key:   "vanityinvite_name",
				Value: vanityName,
			},
			{
				Key:   "vanityinvite_channelid",
				Value: channelID,
				Type:  models.EventlogTargetTypeChannel,
			},
		}, false)
	RelaxLog(err)

	err = ResetCachedDiscordInviteByVanityInvite(newVanityInvite)
	RelaxLog(err)

	return nil
}

func logVanityInviteCreation(channelID string, vanityInvite models.VanityInviteEntry) (err error) {
	author, err := GetUser(vanityInvite.SetByUserID)
	if err != nil {
		return err
	}

	newChannel, err := GetChannel(vanityInvite.ChannelID)
	if err != nil {
		newChannel = new(discordgo.Channel)
		newChannel.Name = "N/A"
	}

	newGuild, err := GetGuild(vanityInvite.GuildID)
	if err != nil {
		newGuild = new(discordgo.Guild)
		newGuild.Name = "N/A"
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("@%s#%s created a Custom Invite 🆕:", author.Username, author.Discriminator),
		},
		Color: 0x0FADED,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Vanity Invite ID: %s | Set at %s",
				vanityInvite.ID, vanityInvite.SetAt.Format(time.ANSIC)),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Name",
				Value:  fmt.Sprintf("`%s`", vanityInvite.VanityNamePretty),
				Inline: false,
			},
			{
				Name: "Channel",
				Value: fmt.Sprintf("`#%s`\n`#%s`",
					newChannel.Name, vanityInvite.ChannelID),
				Inline: false,
			},
			{
				Name: "Server",
				Value: fmt.Sprintf("`%s`\n`#%s`",
					newGuild.Name, vanityInvite.GuildID),
				Inline: false,
			},
		},
	}

	if author.Avatar != "" {
		embed.Author.IconURL = author.AvatarURL("")
	}

	_, err = SendEmbed(channelID, embed)
	return err
}

func logVanityInviteDeletion(channelID string, vanityInvite models.VanityInviteEntry) (err error) {
	author, err := GetUser(vanityInvite.SetByUserID)
	if err != nil {
		return err
	}

	newChannel, err := GetChannel(vanityInvite.ChannelID)
	if err != nil {
		newChannel = new(discordgo.Channel)
		newChannel.Name = "N/A"
	}

	newGuild, err := GetGuild(vanityInvite.GuildID)
	if err != nil {
		newGuild = new(discordgo.Guild)
		newGuild.Name = "N/A"
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("@%s#%s deleted a Custom Invite 🚮:", author.Username, author.Discriminator),
		},
		Color: 0x0FADED,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Vanity Invite ID: %s | Set at %s",
				vanityInvite.ID, vanityInvite.SetAt.Format(time.ANSIC)),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Name",
				Value:  fmt.Sprintf("`%s`", vanityInvite.VanityNamePretty),
				Inline: false,
			},
			{
				Name: "Channel",
				Value: fmt.Sprintf("`#%s`\n`#%s`",
					newChannel.Name, vanityInvite.ChannelID),
				Inline: false,
			},
			{
				Name: "Server",
				Value: fmt.Sprintf("`%s`\n`#%s`",
					newGuild.Name, vanityInvite.GuildID),
				Inline: false,
			},
		},
	}

	if author.Avatar != "" {
		embed.Author.IconURL = author.AvatarURL("")
	}

	_, err = SendEmbed(channelID, embed)
	return err
}

func logVanityInviteChange(channelID string, vanityInvite, oldVanityInvite models.VanityInviteEntry) (err error) {
	author, err := GetUser(vanityInvite.SetByUserID)
	if err != nil {
		return err
	}

	newGuild, err := GetGuild(vanityInvite.GuildID)
	if err != nil {
		newGuild = new(discordgo.Guild)
		newGuild.Name = "N/A"
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("@%s#%s changed a Custom Invite 🔄:", author.Username, author.Discriminator),
		},
		Color: 0x0FADED,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Vanity Invite ID: %s | Set at %s",
				vanityInvite.ID, vanityInvite.SetAt.Format(time.ANSIC)),
		},
		Fields: []*discordgo.MessageEmbedField{},
	}

	if vanityInvite.VanityNamePretty != oldVanityInvite.VanityNamePretty {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Name",
			Value:  fmt.Sprintf("`%s` ➡ `%s`", oldVanityInvite.VanityNamePretty, vanityInvite.VanityNamePretty),
			Inline: false,
		})
	}
	if vanityInvite.ChannelID != oldVanityInvite.ChannelID {
		newChannel, err := GetChannel(vanityInvite.ChannelID)
		if err != nil {
			newChannel = new(discordgo.Channel)
			newChannel.Name = "N/A"
		}
		oldChannel, err := GetChannel(oldVanityInvite.ChannelID)
		if err != nil {
			oldChannel = new(discordgo.Channel)
			oldChannel.Name = "N/A"
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: "Channel",
			Value: fmt.Sprintf("`#%s` ➡ `#%s`\n`#%s` ➡ `#%s`",
				oldChannel.Name, newChannel.Name,
				oldVanityInvite.ChannelID, vanityInvite.ChannelID),
			Inline: false,
		})
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name: "Server",
		Value: fmt.Sprintf("`%s`\n`#%s`",
			newGuild.Name, vanityInvite.GuildID),
		Inline: true,
	})

	if author.Avatar != "" {
		embed.Author.IconURL = author.AvatarURL("")
	}

	_, err = SendEmbed(channelID, embed)
	return err
}

func GetVanityUrlByGuildID(guildID string) (entryBucket models.VanityInviteEntry, err error) {
	err = MdbOneWithoutLogging(
		MdbCollection(models.VanityInvitesTable).Find(bson.M{"guildid": guildID}),
		&entryBucket,
	)
	return entryBucket, err
}

func GetVanityUrlByVanityName(vanityName string) (entryBucket models.VanityInviteEntry, err error) {
	err = MdbOneWithoutLogging(
		MdbCollection(models.VanityInvitesTable).Find(bson.M{"vanityname": strings.ToLower(vanityName)}),
		&entryBucket,
	)
	return entryBucket, err
}

func RemoveVanityUrl(vanityInviteEntry models.VanityInviteEntry) error {
	if vanityInviteEntry.ID.Valid() {
		cache.GetLogger().WithField("module", "helpers/vanityinvite").
			WithField("guildID", vanityInviteEntry.GuildID).
			Info("removed the vanity url", vanityInviteEntry.VanityNamePretty)

		err := MDbDelete(models.VanityInvitesTable, vanityInviteEntry.ID)
		if err != nil {
			return err
		}

		go func() {
			logChannelID, _ := GetBotConfigString(models.VanityInviteLogChannelKey)
			if logChannelID != "" {
				err = logVanityInviteDeletion(logChannelID, vanityInviteEntry)
				RelaxLog(err)
			}
		}()

		return nil
	}
	return errors.New("empty vanityName submitted")
}

func UpdateVanityUrl(vanityInviteEntry models.VanityInviteEntry) error {
	if vanityInviteEntry.ID.Valid() {
		err := MDbUpdate(models.VanityInvitesTable, vanityInviteEntry.ID, vanityInviteEntry)
		return err
	}
	return errors.New("empty vanityName submitted")
}

func ResetCachedDiscordInviteByVanityInvite(vanityInviteEntry models.VanityInviteEntry) (err error) {
	cacheCodec := cache.GetRedisCacheCodec()
	key := fmt.Sprintf(models.VanityInvitesInviteRedisKey, vanityInviteEntry.GuildID)

	err = cacheCodec.Delete(key)
	if err != nil {
		if !strings.Contains(err.Error(), "cache: key is missing") {
			RelaxLog(err)
		}
	}

	_, err = GetDiscordInviteByVanityInvite(vanityInviteEntry)
	return err
}

func GetDiscordInviteByVanityInvite(vanityInviteEntry models.VanityInviteEntry) (code string, err error) {
	redisClient := cache.GetRedisClient()
	key := fmt.Sprintf(models.VanityInvitesInviteRedisKey, vanityInviteEntry.GuildID)

	var vanityInviteRedis models.VanityInviteRedisEntry

	cacheResult, err := redisClient.Get(key).Bytes()
	if err == nil {
		err = json.Unmarshal(cacheResult, &vanityInviteRedis)
		if err == nil && time.Now().Before(vanityInviteRedis.ExpiresAt) {

			fmt.Printf("got item from cache: %#v, %s\n", vanityInviteRedis, vanityInviteRedis.ExpiresAt.String()) // TODO
			return vanityInviteRedis.InviteCode, nil
		}
	}

	cache.GetLogger().WithField("module", "vanityinvite").Infof(
		"created a new invite for Guild #%s to Channel #%s", vanityInviteEntry.GuildID, vanityInviteEntry.ChannelID)

	invite, err := cache.GetSession().SessionForGuildS(vanityInviteEntry.GuildID).ChannelInviteCreate(vanityInviteEntry.ChannelID, discordgo.Invite{
		MaxAge: 60 * 60 * 24, // 1 day
		// Unique: true,
	})
	if err != nil {
		return "", err
	}

	createdAtTime, err := invite.CreatedAt.Parse()
	if err != nil {
		createdAtTime = time.Now()
	}

	expireAt := createdAtTime.Add(time.Duration(invite.MaxAge) * time.Second).Add(-3 * time.Hour)

	vanityInviteRedis = models.VanityInviteRedisEntry{
		InviteCode: invite.Code,
		ExpiresAt:  expireAt,
	}

	fmt.Printf("caching item: %#v, %s, %s\n", vanityInviteRedis, expireAt.String(), expireAt.Sub(time.Now()).String()) // TODO

	cacheItem, err := json.Marshal(vanityInviteRedis)
	if err == nil {
		err = redisClient.Set(key, cacheItem, expireAt.Sub(time.Now())).Err()
		RelaxLog(err)
	}

	return invite.Code, nil
}
