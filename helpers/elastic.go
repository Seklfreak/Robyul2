package helpers

import (
	"errors"

	"context"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
)

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
		MessageID:   message.ID,
		Content:     message.Content,
		Attachments: attachments,
		CreatedAt:   GetTimeFromSnowflake(message.ID),
		AuthorID:    message.Author.ID,
		GuildID:     channel.GuildID,
		ChannelID:   message.ChannelID,
		Embeds:      len(message.Embeds),
	}
	_, err = cache.GetElastic().Index().
		Index("robyul-messages").
		Type("message").
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
		Index("robyul-joins").
		Type("join").
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
		Index("robyul-leaves").
		Type("leave").
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
		Index("robyul-reactions").
		Type("reaction").
		BodyJson(elasticLeaveData).
		Do(context.Background())
	return err
}
