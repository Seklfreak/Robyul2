package eventlog

import (
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
)

func auditlogBackfillLoop() {
	defer helpers.Recover()
	defer func() {
		go func() {
			logger().Error("the auditlogBackfillLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			auditlogBackfillLoop()
		}()
	}()

	for {
		time.Sleep(time.Minute * 1)
		start := time.Now()

		redis := cache.GetRedisClient()

		helpers.AuditLogBackfillRequestsLock.Lock()
		channelCreateBackfillGuildIDs, errMembers1 := redis.SMembers(models.AuditLogBackfillTypeChannelCreateRedisSet).Result()
		channelDeleteBackfillGuildIDs, errMembers2 := redis.SMembers(models.AuditLogBackfillTypeChannelDeleteRedisSet).Result()
		roleCreateBackfillGuildIDs, errMembers3 := redis.SMembers(models.AuditLogBackfillTypeRoleCreateRedisSet).Result()
		roleDeleteBackfillGuildIDs, errMembers4 := redis.SMembers(models.AuditLogBackfillTypeRoleDeleteRedisSet).Result()
		banAddBackfillGuildIDs, errMembers5 := redis.SMembers(models.AuditLogBackfillTypeBanAddRedisSet).Result()
		banRemoveBackfillGuildIDs, errMembers6 := redis.SMembers(models.AuditLogBackfillTypeBanRemoveRedisSet).Result()
		memberRemoveBackfillGuildIDs, errMembers7 := redis.SMembers(models.AuditLogBackfillTypeMemberRemoveRedisSet).Result()
		emojiCreateBackfillGuildIDs, errMembers8 := redis.SMembers(models.AuditLogBackfillTypeEmojiCreateRedisSet).Result()
		emojiDeleteBackfillGuildIDs, errMembers9 := redis.SMembers(models.AuditLogBackfillTypeEmojiDeleteRedisSet).Result()
		emojiUpdateBackfillGuildIDs, errMembers10 := redis.SMembers(models.AuditLogBackfillTypeEmojiUpdateRedisSet).Result()
		guildUpdateBackfillGuildIDs, errMembers11 := redis.SMembers(models.AuditLogBackfillTypeGuildUpdateRedisSet).Result()
		channelUpdateBackfillGuildIDs, errMembers12 := redis.SMembers(models.AuditLogBackfillTypeChannelUpdateRedisSet).Result()
		roleUpdateBackfillGuildIDs, errMembers13 := redis.SMembers(models.AuditLogBackfillTypeRoleUpdateRedisSet).Result()
		memberRoleUpdateBackfillGuildIDs, errMembers14 := redis.SMembers(models.AuditLogBackfillTypeMemberRoleUpdateRedisSet).Result()
		_, errDel1 := redis.Del(models.AuditLogBackfillTypeChannelCreateRedisSet).Result()
		_, errDel2 := redis.Del(models.AuditLogBackfillTypeChannelDeleteRedisSet).Result()
		_, errDel3 := redis.Del(models.AuditLogBackfillTypeRoleCreateRedisSet).Result()
		_, errDel4 := redis.Del(models.AuditLogBackfillTypeRoleDeleteRedisSet).Result()
		_, errDel5 := redis.Del(models.AuditLogBackfillTypeBanAddRedisSet).Result()
		_, errDel6 := redis.Del(models.AuditLogBackfillTypeBanRemoveRedisSet).Result()
		_, errDel7 := redis.Del(models.AuditLogBackfillTypeMemberRemoveRedisSet).Result()
		_, errDel8 := redis.Del(models.AuditLogBackfillTypeEmojiCreateRedisSet).Result()
		_, errDel9 := redis.Del(models.AuditLogBackfillTypeEmojiDeleteRedisSet).Result()
		_, errDel10 := redis.Del(models.AuditLogBackfillTypeEmojiUpdateRedisSet).Result()
		_, errDel11 := redis.Del(models.AuditLogBackfillTypeGuildUpdateRedisSet).Result()
		_, errDel12 := redis.Del(models.AuditLogBackfillTypeChannelUpdateRedisSet).Result()
		_, errDel13 := redis.Del(models.AuditLogBackfillTypeRoleUpdateRedisSet).Result()
		_, errDel14 := redis.Del(models.AuditLogBackfillTypeMemberRoleUpdateRedisSet).Result()
		helpers.AuditLogBackfillRequestsLock.Unlock()
		helpers.Relax(errMembers1)
		helpers.Relax(errMembers2)
		helpers.Relax(errMembers3)
		helpers.Relax(errMembers4)
		helpers.Relax(errMembers5)
		helpers.Relax(errMembers6)
		helpers.Relax(errMembers7)
		helpers.Relax(errMembers8)
		helpers.Relax(errMembers9)
		helpers.Relax(errMembers10)
		helpers.Relax(errMembers11)
		helpers.Relax(errMembers12)
		helpers.Relax(errMembers13)
		helpers.Relax(errMembers14)
		helpers.Relax(errDel1)
		helpers.Relax(errDel2)
		helpers.Relax(errDel3)
		helpers.Relax(errDel4)
		helpers.Relax(errDel5)
		helpers.Relax(errDel6)
		helpers.Relax(errDel7)
		helpers.Relax(errDel8)
		helpers.Relax(errDel9)
		helpers.Relax(errDel10)
		helpers.Relax(errDel11)
		helpers.Relax(errDel12)
		helpers.Relax(errDel13)
		helpers.Relax(errDel14)

		var successfulBackfills int

		for _, guildID := range channelCreateBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing channel create backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionChannelCreate, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeChannelCreate, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range channelDeleteBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing channel delete backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionChannelDelete, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeChannelDelete, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range channelUpdateBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing channel update backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionChannelUpdate, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeChannelUpdate, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range memberRoleUpdateBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing member role update backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionMemberRoleUpdate, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeMemberUpdate, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range roleCreateBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing role create backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionRoleCreate, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeRoleCreate, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range roleDeleteBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing role delete backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionRoleDelete, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeRoleDelete, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				options := make([]models.ElasticEventlogOption, 0)

				for _, change := range result.Changes {
					switch change.Key {
					case "color":
						colorValue, _ := change.OldValue.(int)
						if colorValue > 0 {
							options = append(options, models.ElasticEventlogOption{
								Key:   "role_color",
								Value: helpers.GetHexFromDiscordColor(colorValue),
							})
						}
						break
					case "mentionable":
						mentionAbleValue, _ := change.OldValue.(bool)
						options = append(options, models.ElasticEventlogOption{
							Key:   "role_mentionable",
							Value: helpers.StoreBoolAsString(mentionAbleValue),
						})
						break
					case "hoist":
						hoistValue, _ := change.OldValue.(bool)
						options = append(options, models.ElasticEventlogOption{
							Key:   "role_hoist",
							Value: helpers.StoreBoolAsString(hoistValue),
						})
						break
					case "name":
						nameValue, _ := change.OldValue.(string)
						options = append(options, models.ElasticEventlogOption{
							Key:   "role_name",
							Value: nameValue,
						})
						break
					case "permissions":
						// TODO: handle permissions, example, change.OldValue = 104324161
						break
					}
				}

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						options,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range channelDeleteBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing channel delete backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionChannelDelete, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeChannelDelete, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range banAddBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing ban add backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionMemberBanAdd, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeBanAdd, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}

				elasticItems, err = helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeMemberLeave, true)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						[]models.ElasticEventlogOption{{
							Key:   "member_leave_type",
							Value: "ban",
						}},
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range banRemoveBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing ban remove backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionMemberBanRemove, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeBanRemove, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range memberRemoveBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing member remove backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionMemberKick, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeMemberLeave, true)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						[]models.ElasticEventlogOption{{
							Key:   "member_leave_type",
							Value: "kick",
						}},
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range emojiCreateBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing emoji create backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionEmojiCreate, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeEmojiCreate, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range emojiDeleteBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing emoji delete backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionEmojiDelete, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeEmojiDelete, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range emojiUpdateBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing emoji update backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionEmojiUpdate, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeEmojiUpdate, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range guildUpdateBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing guild update backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionGuildUpdate, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeGuildUpdate, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		for _, guildID := range roleUpdateBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing role update backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionRoleUpdate, 5)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeRoleUpdate, false)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.EventlogLogUpdate(
						elasticItems[0].ElasticID,
						result.UserID,
						nil,
						nil,
						result.Reason,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		elapsed := time.Since(start)
		logger().Infof("did %d audit log backfills, %d entries backfilled, took %s",
			len(channelCreateBackfillGuildIDs)+len(channelDeleteBackfillGuildIDs)+
				len(roleCreateBackfillGuildIDs)+len(roleDeleteBackfillGuildIDs)+
				len(banAddBackfillGuildIDs)+len(banRemoveBackfillGuildIDs)+
				len(memberRemoveBackfillGuildIDs)+
				len(emojiCreateBackfillGuildIDs)+len(emojiDeleteBackfillGuildIDs)+len(emojiUpdateBackfillGuildIDs)+
				len(guildUpdateBackfillGuildIDs)+len(channelUpdateBackfillGuildIDs)+len(roleUpdateBackfillGuildIDs)+
				len(memberRoleUpdateBackfillGuildIDs),
			successfulBackfills, elapsed)
		metrics.EventlogAuditLogBackfillTime.Set(elapsed.Seconds())
	}
}

func shouldBackfill(guildID string) (do bool) {
	if helpers.GuildSettingsGetCached(guildID).EventlogDisabled {
		return false
	}

	if helpers.GetMemberPermissions(guildID, cache.GetSession().State.User.ID)&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator &&
		helpers.GetMemberPermissions(guildID, cache.GetSession().State.User.ID)&discordgo.PermissionViewAuditLogs != discordgo.PermissionViewAuditLogs {
		return false
	}

	return true
}
