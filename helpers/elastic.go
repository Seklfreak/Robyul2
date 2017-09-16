package helpers

import (
	"errors"

	"context"

	"time"

	"sync"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
)

var lastPresenceUpdates map[string]models.ElasticPresenceUpdate
var lastPresenceUpdatesLock = sync.RWMutex{}

func ElasticOnMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	go func() {
		defer Recover()

		err := ElasticAddMessage(message.Message)
		Relax(err)
	}()
}

func ElasticOnGuildMemberAdd(session *discordgo.Session, member *discordgo.GuildMemberAdd) {
	go func() {
		defer Recover()

		err := ElasticAddJoin(member.Member)
		Relax(err)
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

	gameType := -1
	var gameName, gameURL, Status string

	if presence.Game != nil && presence.Game.Name != "" {
		gameName = presence.Game.Name
		gameType = presence.Game.Type
		gameURL = presence.Game.URL
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
		CreatedAt: time.Now(),
		UserID:    presence.User.ID,
		GameType:  gameType,
		GameURL:   gameName,
		GameName:  gameURL,
		Status:    Status,
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
		Content:       message.Content,
		ContentLength: len(message.Content),
		Attachments:   attachments,
		CreatedAt:     GetTimeFromSnowflake(message.ID),
		UserID:        message.Author.ID,
		GuildID:       channel.GuildID,
		ChannelID:     message.ChannelID,
		Embeds:        len(message.Embeds),
	}
	_, err = cache.GetElastic().Index().
		Index(models.ElasticIndex).
		Type(models.ElasticTypeMessage).
		BodyJson(elasticMessageData).
		Do(context.Background())
	return err
}

func ElasticAddJoin(member *discordgo.Member) error {
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
		CreatedAt: joinedAt,
		GuildID:   member.GuildID,
		UserID:    member.User.ID,
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
	_, err = cache.GetElastic().Index().
		Index(models.ElasticIndex).
		Type(models.ElasticTypeReaction).
		BodyJson(elasticLeaveData).
		Do(context.Background())
	return err
}
