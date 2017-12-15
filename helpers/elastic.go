package helpers

import (
	"errors"

	"context"

	"time"

	"sync"

	"encoding/json"

	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"gopkg.in/olivere/elastic.v5"
)

var lastPresenceUpdates map[string]models.ElasticPresenceUpdate
var lastPresenceUpdatesLock = sync.RWMutex{}

func ElasticOnMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	channel, err := GetChannelWithoutApi(message.ChannelID)
	if err != nil {
		return
	}

	if IsBlacklistedGuild(channel.GuildID) {
		return
	}

	if IsLimitedGuild(channel.GuildID) {
		return
	}

	go func() {
		defer Recover()

		err := ElasticAddMessage(message.Message)
		Relax(err)
	}()
}

func ElasticOnMessageUpdate(session *discordgo.Session, message *discordgo.MessageUpdate) {
	channel, err := GetChannelWithoutApi(message.ChannelID)
	if err != nil {
		return
	}

	if IsBlacklistedGuild(channel.GuildID) {
		return
	}

	if IsLimitedGuild(channel.GuildID) {
		return
	}

	go func() {
		defer Recover()

		err := ElasticUpdateMessage(message.Message)
		if err != nil {
			if !strings.Contains(err.Error(), "unable to find elastic message") {
				Relax(err)
			}
		}
	}()
}

func ElasticOnMessageDelete(session *discordgo.Session, message *discordgo.MessageDelete) {
	channel, err := GetChannelWithoutApi(message.ChannelID)
	if err != nil {
		return
	}

	if IsBlacklistedGuild(channel.GuildID) {
		return
	}

	if IsLimitedGuild(channel.GuildID) {
		return
	}

	go func() {
		defer Recover()

		err := ElasticDeleteMessage(message.Message)
		if err != nil {
			if !strings.Contains(err.Error(), "unable to find elastic message") {
				Relax(err)
			}
		}
	}()
}

func ElasticOnGuildMemberRemove(session *discordgo.Session, member *discordgo.GuildMemberRemove) {
	go func() {
		defer Recover()

		err := ElasticAddLeave(member.Member)
		Relax(err)
	}()
}

func ElasticOnReactionAdd(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
	channel, err := GetChannelWithoutApi(reaction.ChannelID)
	if err != nil {
		return
	}

	if IsBlacklistedGuild(channel.GuildID) {
		return
	}

	if IsLimitedGuild(channel.GuildID) {
		return
	}

	go func() {
		defer Recover()

		err := ElasticAddReaction(reaction.MessageReaction)
		Relax(err)
	}()
}

func ElasticOnPresenceUpdate(session *discordgo.Session, presence *discordgo.PresenceUpdate) {
	go func() {
		defer Recover()

		err := ElasticAddPresenceUpdate(&presence.Presence)
		Relax(err)
	}()
}

func ElasticAddPresenceUpdate(presence *discordgo.Presence) error {
	if !cache.HasElastic() {
		return errors.New("no elastic client")
	}

	if presence.User == nil || presence.User.ID == "" {
		return nil
	}

	var gameName, gameType, gameURL, Status string

	if presence.Game != nil && presence.Game.Name != "" {
		gameName = presence.Game.Name
		gameURL = presence.Game.URL
		switch presence.Game.Type {
		case discordgo.GameTypeGame:
			gameType = "game"
			break
		case discordgo.GameTypeStreaming:
			gameType = "streaming"
		}
	}

	if presence.Status != "" {
		switch presence.Status {
		case discordgo.StatusOffline, discordgo.StatusInvisible:
			Status = "offline"
		case discordgo.StatusDoNotDisturb:
			Status = "dnd"
		case discordgo.StatusIdle:
			Status = "idle"
		case discordgo.StatusOnline:
			Status = "online"
		}
	}

	if gameName == "" && Status == "" {
		return nil
	}

	elasticPresenceUpdate := models.ElasticPresenceUpdate{
		CreatedAt:  time.Now(),
		UserID:     presence.User.ID,
		GameTypeV2: gameType,
		GameURL:    gameURL,
		GameName:   gameName,
		Status:     Status,
	}

	updatePresence := true
	lastPresenceUpdatesLock.Lock()
	if lastPresenceUpdates == nil {
		lastPresenceUpdates = make(map[string]models.ElasticPresenceUpdate, 0)
	}
	lastPresenceUpdatesLock.Unlock()

	lastPresenceUpdatesLock.RLock()
	if lastPresence, ok := lastPresenceUpdates[presence.User.ID]; ok {
		if elasticPresenceUpdate.GameType == lastPresence.GameType &&
			elasticPresenceUpdate.GameURL == lastPresence.GameURL &&
			elasticPresenceUpdate.GameName == lastPresence.GameName &&
			elasticPresenceUpdate.Status == lastPresence.Status {
			updatePresence = false
		}
	}
	lastPresenceUpdatesLock.RUnlock()

	if updatePresence {
		lastPresenceUpdatesLock.Lock()
		lastPresenceUpdates[presence.User.ID] = elasticPresenceUpdate
		lastPresenceUpdatesLock.Unlock()
		_, err := cache.GetElastic().Index().
			Index(models.ElasticIndex).
			Type(models.ElasticTypePresenceUpdate).
			BodyJson(elasticPresenceUpdate).
			Do(context.Background())
		return err
	}
	return nil
}

func ElasticAddMessage(message *discordgo.Message) error {
	if !cache.HasElastic() {
		return errors.New("no elastic client")
	}

	channel, err := GetChannel(message.ChannelID)
	if err != nil {
		return err
	}
	attachments := make([]string, 0)
	if len(message.Attachments) > 0 {
		for _, attachment := range message.Attachments {
			attachments = append(attachments, attachment.URL)
		}
	}

	elasticMessageData := models.ElasticMessage{
		MessageID:     message.ID,
		Content:       []string{message.Content},
		ContentLength: len(message.Content),
		Attachments:   attachments,
		CreatedAt:     GetTimeFromSnowflake(message.ID),
		UserID:        message.Author.ID,
		GuildID:       channel.GuildID,
		ChannelID:     message.ChannelID,
		Embeds:        len(message.Embeds),
	}

	if GuildSettingsGetCached(channel.GuildID).ChatlogDisabled {
		elasticMessageData.Content = []string{}
		elasticMessageData.Attachments = []string{}
		elasticMessageData.ContentLength = 0
		elasticMessageData.UserID = ""
		elasticMessageData.Embeds = 0
	}

	_, err = cache.GetElastic().Index().
		Index(models.ElasticIndex).
		Type(models.ElasticTypeMessage).
		BodyJson(elasticMessageData).
		Do(context.Background())
	return err
}

func ElasticUpdateMessage(message *discordgo.Message) error {
	if !cache.HasElastic() {
		return errors.New("no elastic client")
	}

	channel, err := GetChannel(message.ChannelID)
	if err != nil {
		return err
	}

	if GuildSettingsGetCached(channel.GuildID).ChatlogDisabled {
		return nil
	}

	elasticID, oldElasticMessage, err := getElasticMessage(message.ID, channel.ID, channel.GuildID)
	if err != nil {
		return err
	}

	if len(oldElasticMessage.Content) > 0 && oldElasticMessage.Content[len(oldElasticMessage.Content)-1] == message.Content {
		return nil
	}

	_, err = cache.GetElastic().Update().Index(models.ElasticIndex).Type(models.ElasticTypeMessage).Id(elasticID).
		Script(elastic.
			NewScript("ctx._source.Content.add(params.newContent)").
			Param("newContent", message.Content).
			Lang("painless")).
		Upsert(map[string]interface{}{"newContent": 0}).
		Do(context.Background())
	if err != nil {
		cache.GetLogger().WithField("module", "elastic").Errorf("failed to update message, elasticID: %s, newContent: %s, error: %s", elasticID, message.Content, err.Error())
		return err
	}

	return nil
}

func ElasticDeleteMessage(message *discordgo.Message) error {
	if !cache.HasElastic() {
		return errors.New("no elastic client")
	}

	channel, err := GetChannel(message.ChannelID)
	if err != nil {
		return err
	}

	if GuildSettingsGetCached(channel.GuildID).ChatlogDisabled {
		return nil
	}

	elasticID, _, err := getElasticMessage(message.ID, channel.ID, channel.GuildID)
	if err != nil {
		return err
	}

	_, err = cache.GetElastic().Update().Index(models.ElasticIndex).Type(models.ElasticTypeMessage).Id(elasticID).
		Script(elastic.
			NewScript("ctx._source.Deleted = params.deleted").
			Param("deleted", true).
			Lang("painless")).
		Upsert(map[string]interface{}{"deleted": 0}).
		Do(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func ElasticAddJoin(member *discordgo.Member, usedInvite, usedVanityName string) error {
	if !cache.HasElastic() {
		return errors.New("no elastic client")
	}

	var err error
	joinedAt := time.Now()
	if member.JoinedAt != "" {
		joinedAt, err = discordgo.Timestamp(member.JoinedAt).Parse()
		if err != nil {
			return err
		}
	}

	elasticJoinData := models.ElasticJoin{
		CreatedAt:      joinedAt,
		GuildID:        member.GuildID,
		UserID:         member.User.ID,
		UsedInviteCode: usedInvite,
		VanityInvite:   usedVanityName,
	}

	if GuildSettingsGetCached(member.GuildID).ChatlogDisabled {
		elasticJoinData.UserID = ""
	}

	_, err = cache.GetElastic().Index().
		Index(models.ElasticIndex).
		Type(models.ElasticTypeJoin).
		BodyJson(elasticJoinData).
		Do(context.Background())
	return err
}

func ElasticAddLeave(member *discordgo.Member) error {
	if !cache.HasElastic() {
		return errors.New("no elastic client")
	}

	var err error
	joinedAt := time.Now()
	if member.JoinedAt != "" {
		joinedAt, err = discordgo.Timestamp(member.JoinedAt).Parse()
		if err != nil {
			return err
		}
	}

	elasticLeaveData := models.ElasticLeave{
		CreatedAt: joinedAt,
		GuildID:   member.GuildID,
		UserID:    member.User.ID,
	}

	if GuildSettingsGetCached(member.GuildID).ChatlogDisabled {
		elasticLeaveData.UserID = ""
	}

	_, err = cache.GetElastic().Index().
		Index(models.ElasticIndex).
		Type(models.ElasticTypeLeave).
		BodyJson(elasticLeaveData).
		Do(context.Background())
	return err
}

func ElasticAddReaction(reaction *discordgo.MessageReaction) error {
	if !cache.HasElastic() {
		return errors.New("no elastic client")
	}

	var err error
	channel, err := GetChannel(reaction.ChannelID)
	if err != nil {
		return err
	}

	elasticLeaveData := models.ElasticReaction{
		CreatedAt: time.Now(),
		UserID:    reaction.UserID,
		MessageID: reaction.MessageID,
		ChannelID: reaction.ChannelID,
		GuildID:   channel.GuildID,
		EmojiID:   reaction.Emoji.ID,
		EmojiName: reaction.Emoji.Name,
	}

	if GuildSettingsGetCached(channel.GuildID).ChatlogDisabled {
		elasticLeaveData.UserID = ""
		elasticLeaveData.MessageID = ""
		elasticLeaveData.EmojiID = ""
		elasticLeaveData.EmojiName = ""
	}

	_, err = cache.GetElastic().Index().
		Index(models.ElasticIndex).
		Type(models.ElasticTypeReaction).
		BodyJson(elasticLeaveData).
		Do(context.Background())
	return err
}

func ElasticAddVanityInviteClick(vanityInvite models.VanityInviteEntry, referer string) error {
	if !cache.HasElastic() {
		return errors.New("no elastic client")
	}

	if vanityInvite.VanityName == "" || vanityInvite.GuildID == "" {
		return errors.New("invalid vanityinvite entry submitted")
	}

	var err error

	elasticVanityInviteClickData := models.ElasticVanityInviteClick{
		CreatedAt:        time.Now(),
		VanityInviteName: vanityInvite.VanityName,
		GuildID:          vanityInvite.GuildID,
		Referer:          referer,
	}

	_, err = cache.GetElastic().Index().
		Index(models.ElasticIndex).
		Type(models.ElasticTypeVanityInviteClick).
		BodyJson(elasticVanityInviteClickData).
		Do(context.Background())
	return err
}

func GetMinTimeForInterval(interval string, count int) (minTime time.Time) {
	switch interval {
	case "second":
		minTime = time.Now().Add(-1 * time.Duration(count) * time.Second)
		break
	case "minute":
		minTime = time.Now().Add(-1 * time.Duration(count) * time.Minute)
		break
	case "hour":
		minTime = time.Now().Add(-1 * time.Duration(count) * time.Hour)
		break
	case "day":
		minTime = time.Now().Add(-1 * time.Duration(count) * (time.Hour * 24))
		break
	case "week":
		minTime = time.Now().Add(-1 * time.Duration(count) * (time.Hour * 24 * 7))
		break
	case "month":
		minTime = time.Now().Add(-1 * time.Duration(count) * (time.Hour * 24 * 7 * 31))
		break
	case "quarter":
		minTime = time.Now().Add(-1 * time.Duration(count) * (time.Hour * 24 * 7 * 31 * 3))
		break
	case "year":
		minTime = time.Now().Add(-1 * time.Duration(count) * (time.Hour * 24 * 7 * 365))
		break
	}
	return minTime
}

func getElasticMessage(messageID, channelID, guildID string) (elasticID string, message models.ElasticMessage, err error) {
	termQuery := elastic.NewQueryStringQuery("_type:" + models.ElasticTypeMessage + " AND GuildID:" + guildID + " AND ChannelID:" + channelID + " AND MessageID:" + messageID)
	searchResult, err := cache.GetElastic().Search().
		Index(models.ElasticIndex).
		Query(termQuery).
		Size(1).
		Sort("CreatedAt", true).
		Do(context.Background())
	if err != nil {
		return "", message, err
	}

	if err != nil {
		return "", message, err
	}

	for _, item := range searchResult.Hits.Hits {
		if item == nil {
			continue
		}

		message = UnmarshalElasticMessage(item)
		if message.MessageID == "" {
			return "", message, errors.New("unable to get message")
		}

		if message.MessageID == messageID {
			return item.Id, message, nil
		}
	}

	return "", message, errors.New("unable to find elastic message")
}

func UnmarshalElasticMessage(item *elastic.SearchHit) (result models.ElasticMessage) {
	if item == nil {
		return result
	}

	err := json.Unmarshal(*item.Source, &result)
	if err != nil {
		var legacyM models.ElasticLegacyMessage
		err := json.Unmarshal(*item.Source, &legacyM)
		if err != nil {
			return result
		}

		result = models.ElasticMessage{
			CreatedAt:     legacyM.CreatedAt,
			MessageID:     legacyM.MessageID,
			Content:       []string{legacyM.Content},
			ContentLength: legacyM.ContentLength,
			Attachments:   legacyM.Attachments,
			UserID:        legacyM.UserID,
			GuildID:       legacyM.GuildID,
			ChannelID:     legacyM.ChannelID,
			Embeds:        legacyM.Embeds,
			Deleted:       false,
		}

		return result
	}

	return result
}
