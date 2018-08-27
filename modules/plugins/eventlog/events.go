package eventlog

import (
	"strconv"
	"time"

	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/json-iterator/go"
)

func (h *Handler) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
	if !strings.Contains(content, "discord.gg/") && !strings.Contains(content, "discordapp.com/invite/") {
		return
	}

	invitesCodes := helpers.ExtractInviteCodes(content)
	if len(invitesCodes) <= 0 {
		return
	}

	createdAt, err := msg.Timestamp.Parse()
	if err != nil {
		createdAt = time.Now()
	}

	channel, err := helpers.GetChannelWithoutApi(msg.ChannelID)
	helpers.Relax(err)

	postedInviteGuildIDs := make([]string, 0)
	postedInviteGuildNames := make([]string, 0)
	postedInviteGuildMemberCounts := make([]string, 0)
	postedInviteChannelIDs := make([]string, 0)
	postedInviteInviterUserIDs := make([]string, 0)
	for _, inviteCode := range invitesCodes {
		invite, err := cache.GetSession().InviteWithCounts(inviteCode)
		if err == nil && invite != nil && invite.Guild != nil {
			postedInviteGuildIDs = append(postedInviteGuildIDs, invite.Guild.ID)
			postedInviteGuildNames = append(postedInviteGuildNames, invite.Guild.Name)
			postedInviteGuildMemberCounts = append(postedInviteGuildMemberCounts,
				strconv.Itoa(invite.ApproximatePresenceCount)+"/"+strconv.Itoa(invite.ApproximateMemberCount),
			)
			if invite.Channel != nil {
				postedInviteChannelIDs = append(postedInviteChannelIDs, invite.Channel.ID)
			}
			if invite.Inviter != nil {
				postedInviteInviterUserIDs = append(postedInviteInviterUserIDs, invite.Inviter.ID)
			}
		} else {
			postedInviteGuildIDs = append(postedInviteGuildIDs, "N/A")
			postedInviteGuildNames = append(postedInviteGuildNames, "N/A")
			postedInviteGuildMemberCounts = append(postedInviteGuildMemberCounts, "N/A")
			postedInviteChannelIDs = append(postedInviteChannelIDs, "N/A")
			postedInviteInviterUserIDs = append(postedInviteInviterUserIDs, "N/A")
		}
	}

	_, err = helpers.EventlogLog(
		createdAt,
		channel.GuildID,
		msg.ChannelID,
		models.EventlogTargetTypeChannel,
		msg.Author.ID,
		models.EventlogTypeInvitePosted,
		"",
		nil,
		[]models.ElasticEventlogOption{
			{
				Key:   "invite_code",
				Value: strings.Join(invitesCodes, ";"),
				Type:  models.EventlogTargetTypeInviteCode,
			},
			{
				Key:   "invite_guildid",
				Value: strings.Join(postedInviteGuildIDs, ";"),
				Type:  models.EventlogTargetTypeGuild,
			},
			{
				Key:   "invite_guildname",
				Value: strings.Join(postedInviteGuildNames, ";"),
			},
			{
				Key:   "invite_guildmembercount",
				Value: strings.Join(postedInviteGuildMemberCounts, ";"),
			},
			{
				Key:   "invite_channelid",
				Value: strings.Join(postedInviteChannelIDs, ";"),
				Type:  models.EventlogTargetTypeChannel,
			},
			{
				Key:   "invite_inviterid",
				Value: strings.Join(postedInviteInviterUserIDs, ";"),
				Type:  models.EventlogTargetTypeUser,
			},
		},
		false,
	)
	helpers.RelaxLog(err)
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

		added, err := helpers.EventlogLog(leftAt, member.GuildID, member.User.ID, models.EventlogTargetTypeUser, "", models.EventlogTypeMemberLeave, "", nil, nil, false)
		helpers.RelaxLog(err)
		if added {
			err := helpers.RequestAuditLogBackfill(member.GuildID, models.AuditLogBackfillTypeMemberRemove, "")
			helpers.RelaxLog(err)
		}
	}()
}

func (h *Handler) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
	// only react to ↩
	if reaction.Emoji.Name != "↩" {
		return
	}

	channel, err := helpers.GetChannel(reaction.ChannelID)
	if err != nil {
		return
	}

	// skip non mods
	if !helpers.IsModByID(channel.GuildID, reaction.UserID) {
		return
	}

	// check if happend in log channel
	var logChannel bool
	for _, logChannelID := range helpers.GuildSettingsGetCached(channel.GuildID).EventlogChannelIDs {
		if logChannelID == reaction.ChannelID {
			logChannel = true
		}
	}
	if !logChannel {
		return
	}

	err = Container.Drain(1, reaction.UserID)
	if err != nil {
		cache.GetSession().MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
		helpers.SendMessage(reaction.ChannelID, "<@"+reaction.UserID+"> You are undoing too fast.\nPlease wait a bit.")
		return
	}

	// get target message
	message, err := helpers.GetMessage(reaction.ChannelID, reaction.MessageID)
	if err != nil {
		return
	}

	// skip if target message is by robyul and has embeds
	if message.Author.ID != cache.GetSession().State.User.ID || len(message.Embeds) <= 0 {
		return
	}

	// try to find correct embed
	var targetEmbed *discordgo.MessageEmbed
	for _, embed := range message.Embeds {
		if embed != nil && embed.Footer != nil && strings.HasPrefix(embed.Footer.Text, "#") {
			targetEmbed = embed
		}
	}

	if targetEmbed == nil {
		return
	}

	// try to get eventlog ID
	ID := strings.TrimPrefix(strings.SplitN(targetEmbed.Footer.Text, " ", 2)[0], "#")

	// try to get eventlog Item
	eventlogItem, err := helpers.ElasticGetEventlog(ID)
	if err != nil {
		return
	}

	if !helpers.CanRevert(*eventlogItem) {
		return
	}

	err = helpers.Revert(ID, reaction.UserID, *eventlogItem)
	if err != nil {
		cache.GetSession().MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
		helpers.SendMessage(reaction.ChannelID, "<@"+reaction.UserID+"> Error reverting change: "+err.Error())
	} else {
		cache.GetSession().MessageReactionsRemoveAll(reaction.ChannelID, reaction.MessageID)
	}
}

func (h *Handler) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

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

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_type",
			Value: strconv.Itoa(int(channel.Type)),
			Type:  models.EventlogTargetTypeChannelType,
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_topic",
			Value: channel.Topic,
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_nsfw",
			Value: helpers.StoreBoolAsString(channel.NSFW),
		})

		if channel.Bitrate > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_bitrate",
				Value: strconv.Itoa(channel.Bitrate),
			})
		}

		/*
			if channel.Position > 0 {
				options = append(options, models.ElasticEventlogOption{
					Key:   "channel_position",
					Value: strconv.Itoa(channel.Position),
				})
			}
		*/

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_parentid",
			Value: channel.ParentID,
			Type:  models.EventlogTargetTypeChannel,
		})

		/*
			TODO: handle permission overwrites
			options = append(options, models.ElasticEventlogOption{
				Key:   "permission_overwrites",
				Value: channel.PermissionOverwrites,
			})
		*/

		added, err := helpers.EventlogLog(leftAt, channel.GuildID, channel.ID, models.EventlogTargetTypeChannel, "", models.EventlogTypeChannelCreate, "", nil, options, true)
		helpers.RelaxLog(err)
		if added {
			err := helpers.RequestAuditLogBackfill(channel.GuildID, models.AuditLogBackfillTypeChannelCreate, "")
			helpers.RelaxLog(err)
		}
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

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_type",
			Value: strconv.Itoa(int(channel.Type)),
			Type:  models.EventlogTargetTypeChannelType,
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_topic",
			Value: channel.Topic,
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_nsfw",
			Value: helpers.StoreBoolAsString(channel.NSFW),
		})

		if channel.Bitrate > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "channel_bitrate",
				Value: strconv.Itoa(channel.Bitrate),
			})
		}

		/*
			if channel.Position > 0 {
				options = append(options, models.ElasticEventlogOption{
					Key:   "channel_position",
					Value: strconv.Itoa(channel.Position),
				})
			}
		*/

		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_parentid",
			Value: channel.ParentID,
			Type:  models.EventlogTargetTypeChannel,
		})

		var channelOverwrites string
		for _, overwrite := range channel.PermissionOverwrites {
			channelOverwriteText, err := jsoniter.MarshalToString(overwrite)
			helpers.RelaxLog(err)
			if err == nil {
				channelOverwrites += channelOverwriteText + ";"
			}
		}
		options = append(options, models.ElasticEventlogOption{
			Key:   "channel_permissionoverwrites",
			Value: channelOverwrites,
			Type:  models.EventlogTargetTypePermissionOverwrite,
		})

		added, err := helpers.EventlogLog(leftAt, channel.GuildID, channel.ID, models.EventlogTargetTypeChannel, "", models.EventlogTypeChannelDelete, "", nil, options, true)
		helpers.RelaxLog(err)
		if added {
			err := helpers.RequestAuditLogBackfill(channel.GuildID, models.AuditLogBackfillTypeChannelDelete, "")
			helpers.RelaxLog(err)
		}
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
			Value: helpers.StoreBoolAsString(role.Role.Managed),
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "role_mentionable",
			Value: helpers.StoreBoolAsString(role.Role.Mentionable),
		})

		options = append(options, models.ElasticEventlogOption{
			Key:   "role_hoist",
			Value: helpers.StoreBoolAsString(role.Role.Hoist),
		})

		if role.Role.Color > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "role_color",
				Value: helpers.GetHexFromDiscordColor(role.Role.Color),
			})
		}

		/*
			options = append(options, models.ElasticEventlogOption{
				Key:   "role_position",
				Value: strconv.Itoa(role.Role.Position),
			})
		*/

		// TODO: store permissions role.Role.Permissions

		added, err := helpers.EventlogLog(leftAt, role.GuildID, role.Role.ID, models.EventlogTargetTypeRole, "", models.EventlogTypeRoleCreate, "", nil, options, true)
		helpers.RelaxLog(err)
		if added {
			err := helpers.RequestAuditLogBackfill(role.GuildID, models.AuditLogBackfillTypeRoleCreate, "")
			helpers.RelaxLog(err)
		}
	}()
}

func (h *Handler) OnGuildRoleDelete(session *discordgo.Session, role *discordgo.GuildRoleDelete) {
	go func() {
		defer helpers.Recover()

		leftAt := time.Now()

		added, err := helpers.EventlogLog(leftAt, role.GuildID, role.RoleID, models.EventlogTargetTypeRole, "", models.EventlogTypeRoleDelete, "", nil, nil, true)
		helpers.RelaxLog(err)
		if added {
			err := helpers.RequestAuditLogBackfill(role.GuildID, models.AuditLogBackfillTypeRoleDelete, "")
			helpers.RelaxLog(err)
		}
	}()
}

func (h *Handler) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {
	if helpers.GetMemberPermissions(user.GuildID, cache.GetSession().State.User.ID)&discordgo.PermissionBanMembers != discordgo.PermissionBanMembers &&
		helpers.GetMemberPermissions(user.GuildID, cache.GetSession().State.User.ID)&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
		return
	}

	go func() {
		defer helpers.Recover()

		leftAt := time.Now()

		added, err := helpers.EventlogLog(leftAt, user.GuildID, user.User.ID, models.EventlogTargetTypeUser, "", models.EventlogTypeBanAdd, "", nil, nil, true)
		helpers.RelaxLog(err)
		if added {
			err := helpers.RequestAuditLogBackfill(user.GuildID, models.AuditLogBackfillTypeBanAdd, "")
			helpers.RelaxLog(err)
		}
	}()
}

func (h *Handler) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		leftAt := time.Now()

		added, err := helpers.EventlogLog(leftAt, user.GuildID, user.User.ID, models.EventlogTargetTypeUser, "", models.EventlogTypeBanRemove, "", nil, nil, true)
		helpers.RelaxLog(err)
		if added {
			err := helpers.RequestAuditLogBackfill(user.GuildID, models.AuditLogBackfillTypeBanRemove, "")
			helpers.RelaxLog(err)
		}
	}()
}

/*
TODO: cache webhooks, monitor changes
func (h *Handler) OnWebhooksUpdate(user *discordgo.WebhooksUpdate, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()
	}()
}
*/
