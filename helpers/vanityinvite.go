package helpers

import (
	"time"

	"regexp"

	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	redisCache "github.com/go-redis/cache"
	"github.com/gorethink/gorethink"
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
		return nil
	}

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

	insert := gorethink.Table(models.VanityInvitesTable).Insert(newVanityInvite)
	_, err = insert.RunWrite(GetDB())
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

func GetVanityUrlByID(id string) (entryBucket models.VanityInviteEntry, err error) {
	listCursor, err := gorethink.Table(models.VanityInvitesTable).Get(id).Run(GetDB())
	if err != nil {
		return entryBucket, err
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)
	return entryBucket, err
}

func GetVanityUrlByGuildID(guildID string) (entryBucket models.VanityInviteEntry, err error) {
	listCursor, err := gorethink.Table(models.VanityInvitesTable).Filter(
		gorethink.Row.Field("guild_id").Eq(guildID),
	).Run(GetDB())
	if err != nil {
		return entryBucket, err
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)
	return entryBucket, err
}

func GetVanityUrlByVanityName(vanityName string) (entryBucket models.VanityInviteEntry, err error) {
	listCursor, err := gorethink.Table(models.VanityInvitesTable).Filter(
		gorethink.Row.Field("vanity_name").Eq(strings.ToLower(vanityName)),
	).Run(GetDB())
	if err != nil {
		return entryBucket, err
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)
	return entryBucket, err
}

func RemoveVanityUrl(vanityInviteEntry models.VanityInviteEntry) error {
	if vanityInviteEntry.ID != "" {
		cache.GetLogger().WithField("module", "helpers/vanityinvite").
			WithField("guildID", vanityInviteEntry.GuildID).
			Info("removed the vanity url", vanityInviteEntry.VanityNamePretty)

		_, err := gorethink.Table(models.VanityInvitesTable).Get(vanityInviteEntry.ID).Delete().RunWrite(GetDB())
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
	if vanityInviteEntry.ID != "" {
		_, err := gorethink.Table(models.VanityInvitesTable).Update(vanityInviteEntry).Run(GetDB())
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
	cacheCodec := cache.GetRedisCacheCodec()
	key := fmt.Sprintf(models.VanityInvitesInviteRedisKey, vanityInviteEntry.GuildID)

	var vanityInviteRedis models.VanityInviteRedisEntry
	if err = cacheCodec.Get(key, &vanityInviteRedis); err == nil {
		if time.Now().Before(vanityInviteRedis.ExpiresAt) {
			return vanityInviteRedis.InviteCode, nil
		}
	}

	cache.GetLogger().WithField("module", "vanityinvite").Infof(
		"created a new invite for Guild # to Channel #", vanityInviteEntry.GuildID, vanityInviteEntry.ChannelID)

	invite, err := cache.GetSession().ChannelInviteCreate(vanityInviteEntry.ChannelID, discordgo.Invite{
		MaxAge: 60 * 60 * 24, // 1 day
	})
	if err != nil {
		return "", err
	}

	vanityInviteRedis = models.VanityInviteRedisEntry{
		InviteCode: invite.Code,
		ExpiresAt:  time.Now().Add(time.Hour * 23),
	}

	err = cacheCodec.Set(&redisCache.Item{
		Key:        key,
		Object:     vanityInviteRedis,
		Expiration: time.Hour * 23,
	})
	RelaxLog(err)

	return invite.Code, nil
}
