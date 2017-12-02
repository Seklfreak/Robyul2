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
		vanityEntryByGuildID.VanityName = strings.ToLower(vanityName)
		vanityEntryByGuildID.VanityNamePretty = vanityName
		vanityEntryByGuildID.ChannelID = channelID
		vanityEntryByGuildID.SetAt = time.Now()
		vanityEntryByGuildID.SetByUserID = userID

		err = ResetCachedDiscordInviteByVanityInvite(vanityEntryByGuildID)
		RelaxLog(err)

		err = UpdateVanityUrl(vanityEntryByGuildID)
		return err
	}

	newVanityInvite := models.VanityInviteEntry{
		GuildID:          guildID,
		VanityName:       strings.ToLower(vanityName),
		VanityNamePretty: vanityName,
		SetByUserID:      userID,
		SetAt:            time.Now(),
		ChannelID:        channelID,
	}

	insert := gorethink.Table(models.VanityInvitesTable).Insert(newVanityInvite)
	_, err = insert.RunWrite(GetDB())
	if err != nil {
		return err
	}

	err = ResetCachedDiscordInviteByVanityInvite(newVanityInvite)
	RelaxLog(err)

	return nil
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
		_, err := gorethink.Table(models.VanityInvitesTable).Get(vanityInviteEntry.ID).Delete().RunWrite(GetDB())
		return err
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
