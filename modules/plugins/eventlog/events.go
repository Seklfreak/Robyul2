package eventlog

import (
	"strconv"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
)

func (h *Handler) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (h *Handler) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (h *Handler) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
	// handled in mod.go (to get invite code)
}

func (h *Handler) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		leftAt := time.Now()

		helpers.EventlogLog(leftAt, member.GuildID, member.User.ID, models.EventlogTargetTypeUser, "", models.EventlogTypeMemberLeave, "", nil, nil, false)
	}()
}

func (h *Handler) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}

func (h *Handler) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}

func (h *Handler) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (h *Handler) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}

func (h *Handler) OnChannelCreate(session *discordgo.Session, channel *discordgo.ChannelCreate) {
	go func() {
		defer helpers.Recover()

		leftAt := time.Now()

		options := make([]models.ElasticEventlogOption, 0)
		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_name",
			Value: channel.Name,
		})

		switch channel.Type {
		case discordgo.ChannelTypeGuildCategory:
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_type",
				Value: "category",
			})
			break
		case discordgo.ChannelTypeGuildText:
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_type",
				Value: "text",
			})
			break
		case discordgo.ChannelTypeGuildVoice:
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_type",
				Value: "voice",
			})
			break
		}

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_topic",
			Value: channel.Topic,
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_nsfw",
			Value: storeBoolAsString(channel.NSFW),
		})

		if channel.Bitrate > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_bitrate",
				Value: strconv.Itoa(channel.Bitrate),
			})
		}

		if channel.Position > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_position",
				Value: strconv.Itoa(channel.Position),
			})
		}

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_parentid",
			Value: channel.ParentID,
		})

		/*
			TODO: handle permission overwrites
			options = append(options, models.ElasticEventlogOption{
				Key:   "permission_overwrites",
				Value: channel.PermissionOverwrites,
			})
		*/

		helpers.EventlogLog(leftAt, channel.GuildID, channel.ID, models.EventlogTargetTypeChannel, "", models.EventlogTypeChannelCreate, "", nil, options, true)

		err := h.requestAuditLogBackfill(channel.GuildID, AuditLogBackfillTypeChannelCreate)
		helpers.RelaxLog(err)
	}()
}

func (h *Handler) OnChannelDelete(session *discordgo.Session, channel *discordgo.ChannelDelete) {
	go func() {
		defer helpers.Recover()

		leftAt := time.Now()

		options := make([]models.ElasticEventlogOption, 0)
		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_name",
			Value: channel.Name,
		})

		switch channel.Type {
		case discordgo.ChannelTypeGuildCategory:
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_type",
				Value: "category",
			})
			break
		case discordgo.ChannelTypeGuildText:
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_type",
				Value: "text",
			})
			break
		case discordgo.ChannelTypeGuildVoice:
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_type",
				Value: "voice",
			})
			break
		}

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_topic",
			Value: channel.Topic,
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_nsfw",
			Value: storeBoolAsString(channel.NSFW),
		})

		if channel.Bitrate > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_bitrate",
				Value: strconv.Itoa(channel.Bitrate),
			})
		}

		if channel.Position > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_position",
				Value: strconv.Itoa(channel.Position),
			})
		}

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_parentid",
			Value: channel.ParentID,
		})

		/*
			TODO: handle permission overwrites
			options = append(options, models.ElasticEventlogOption{
				Key:   "permission_overwrites",
				Value: channel.PermissionOverwrites,
			})
		*/

		helpers.EventlogLog(leftAt, channel.GuildID, channel.ID, models.EventlogTargetTypeChannel, "", models.EventlogTypeChannelDelete, "", nil, options, true)

		err := h.requestAuditLogBackfill(channel.GuildID, AuditLogBackfillTypeChannelDelete)
		helpers.RelaxLog(err)
	}()
}

func (h *Handler) OnGuildRoleCreate(session *discordgo.Session, role *discordgo.GuildRoleCreate) {
	go func() {
		defer helpers.Recover()

		leftAt := time.Now()

		options := make([]models.ElasticEventlogOption, 0)

		options = append(options, models.ElasticEventlogOption{
			Key:   "role_name",
			Value: role.Role.Name,
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "role_managed",
			Value: storeBoolAsString(role.Role.Managed),
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "role_mentionable",
			Value: storeBoolAsString(role.Role.Mentionable),
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "role_hoist",
			Value: storeBoolAsString(role.Role.Hoist),
		})

		if role.Role.Color > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "role_color",
				Value: helpers.GetHexFromDiscordColor(role.Role.Color),
			})
		}

		options = append(options, models.ElasticEventlogOption{
			Key:   "role_position",
			Value: strconv.Itoa(role.Role.Position),
		})

		// TODO: store permissions role.Role.Permissions

		helpers.EventlogLog(leftAt, role.GuildID, role.Role.ID, models.EventlogTargetTypeRole, "", models.EventlogTypeRoleCreate, "", nil, options, true)

		err := h.requestAuditLogBackfill(role.GuildID, AuditLogBackfillTypeRoleCreate)
		helpers.RelaxLog(err)
	}()
}

func (h *Handler) OnGuildRoleDelete(session *discordgo.Session, role *discordgo.GuildRoleDelete) {
	go func() {
		defer helpers.Recover()

		leftAt := time.Now()

		helpers.EventlogLog(leftAt, role.GuildID, role.RoleID, models.EventlogTargetTypeRole, "", models.EventlogTypeRoleDelete, "", nil, nil, true)

		err := h.requestAuditLogBackfill(role.GuildID, AuditLogBackfillTypeRoleDelete)
		helpers.RelaxLog(err)
	}()
}
