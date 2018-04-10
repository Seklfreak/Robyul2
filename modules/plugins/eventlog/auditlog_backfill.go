package eventlog

import (
	"strings"
	"time"

	"encoding/json"

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
		backfills, err := redis.SMembers(models.AuditLogBackfillRedisSet).Result()
		if err != nil {
			helpers.AuditLogBackfillRequestsLock.Unlock()
			helpers.Relax(err)
		}
		_, err = redis.Del(models.AuditLogBackfillRedisSet).Result()
		if err != nil {
			helpers.AuditLogBackfillRequestsLock.Unlock()
			helpers.Relax(err)
		}
		helpers.AuditLogBackfillRequestsLock.Unlock()
		var successfulBackfills int

		for _, backfillMarshalled := range backfills {
			var backfill models.AuditLogBackfillRequest
			err = json.Unmarshal([]byte(backfillMarshalled), &backfill)
			if err != nil {
				helpers.RelaxLog(err)
				continue
			}

			if backfill.GuildID == "" {
				continue
			}

			if !shouldBackfill(backfill.GuildID) {
				continue
			}

			switch backfill.Type {
			case models.AuditLogBackfillTypeChannelCreate:
				logger().Infof("doing channel create backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionChannelCreate, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeChannelCreate, false)
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
				break
			case models.AuditLogBackfillTypeChannelDelete:
				logger().Infof("doing channel delete backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionChannelDelete, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeChannelDelete, false)
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
				break
			case models.AuditLogBackfillTypeChannelUpdate:
				logger().Infof("doing channel update backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionChannelUpdate, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeChannelUpdate, false)
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
				break
			case models.AuditlogBackfillTypeMemberRoleUpdate:
				logger().Infof("doing member role update backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionMemberRoleUpdate, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeMemberUpdate, false)
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
				break
			case models.AuditlogBackfillTypeMemberUpdate:
				logger().Infof("doing member update backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionMemberUpdate, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeMemberUpdate, false)
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
				break
			case models.AuditLogBackfillTypeRoleCreate:
				logger().Infof("doing role create backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionRoleCreate, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeRoleCreate, false)
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
				break
			case models.AuditLogBackfillTypeRoleDelete:
				logger().Infof("doing role delete backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionRoleDelete, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeRoleDelete, false)
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
				break
			case models.AuditLogBackfillTypeBanAdd:
				logger().Infof("doing ban add backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionMemberBanAdd, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeBanAdd, false)
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

					elasticItems, err = helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeMemberLeave, true)
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
				break
			case models.AuditLogBackfillTypeBanRemove:
				logger().Infof("doing ban remove backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionMemberBanRemove, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeBanRemove, false)
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
				break
			case models.AuditLogBackfillTypeMemberRemove:
				logger().Infof("doing member remove backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionMemberKick, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeMemberLeave, true)
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
				break
			case models.AuditLogBackfillTypeEmojiCreate:
				logger().Infof("doing emoji create backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionEmojiCreate, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeEmojiCreate, false)
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
				break
			case models.AuditLogBackfillTypeEmojiDelete:
				logger().Infof("doing emoji delete backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionEmojiDelete, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeEmojiDelete, false)
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
				break
			case models.AuditLogBackfillTypeEmojiUpdate:
				logger().Infof("doing emoji update backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionEmojiUpdate, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeEmojiUpdate, false)
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
				break
			case models.AuditLogBackfillTypeGuildUpdate:
				logger().Infof("doing guild update backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionGuildUpdate, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeGuildUpdate, false)
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
				break
			case models.AuditLogBackfillTypeRoleUpdate:
				logger().Infof("doing role update backfill for guild #%s", backfill.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfill.GuildID, "", "", discordgo.AuditLogActionRoleUpdate, 1)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						continue
					}
				}
				helpers.Relax(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfill.GuildID, result.TargetID, models.EventlogTypeRoleUpdate, false)
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
				break
			}

		}

		elapsed := time.Since(start)
		logger().Infof("did %d audit log backfills, %d entries backfilled, took %s",
			len(backfills),
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
