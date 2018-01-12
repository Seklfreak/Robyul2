package eventlog

import (
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

const (
	AuditLogBackfillTypeChannelCreate AuditLogBackfillType = 1 << iota
	AuditLogBackfillTypeChannelDelete
	AuditLogBackfillTypeRoleCreate
	AuditLogBackfillTypeRoleDelete
	AuditLogBackfillTypeBanAdd
	AuditLogBackfillTypeBanRemove
	AuditLogBackfillTypeMemberRemove
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

		auditLogBackfillRequestsLock.Lock()
		channelCreateBackfillGuildIDs, errMembers1 := redis.SMembers(models.AuditLogBackfillTypeChannelCreateRedisSet).Result()
		channelDeleteBackfillGuildIDs, errMembers2 := redis.SMembers(models.AuditLogBackfillTypeChannelDeleteRedisSet).Result()
		roleCreateBackfillGuildIDs, errMembers3 := redis.SMembers(models.AuditLogBackfillTypeRoleCreateRedisSet).Result()
		roleDeleteBackfillGuildIDs, errMembers4 := redis.SMembers(models.AuditLogBackfillTypeRoleDeleteRedisSet).Result()
		banAddBackfillGuildIDs, errMembers5 := redis.SMembers(models.AuditLogBackfillTypeBanAddRedisSet).Result()
		banRemoveBackfillGuildIDs, errMembers6 := redis.SMembers(models.AuditLogBackfillTypeBanRemoveRedisSet).Result()
		memberRemoveBackfillGuildIDs, errMembers7 := redis.SMembers(models.AuditLogBackfillTypeMemberRemoveRedisSet).Result()
		_, errDel1 := redis.Del(models.AuditLogBackfillTypeChannelCreateRedisSet).Result()
		_, errDel2 := redis.Del(models.AuditLogBackfillTypeChannelDeleteRedisSet).Result()
		_, errDel3 := redis.Del(models.AuditLogBackfillTypeRoleCreateRedisSet).Result()
		_, errDel4 := redis.Del(models.AuditLogBackfillTypeRoleDeleteRedisSet).Result()
		_, errDel5 := redis.Del(models.AuditLogBackfillTypeBanAddRedisSet).Result()
		_, errDel6 := redis.Del(models.AuditLogBackfillTypeBanRemoveRedisSet).Result()
		_, errDel7 := redis.Del(models.AuditLogBackfillTypeMemberRemoveRedisSet).Result()
		auditLogBackfillRequestsLock.Unlock()
		helpers.Relax(errMembers1)
		helpers.Relax(errMembers2)
		helpers.Relax(errMembers3)
		helpers.Relax(errMembers4)
		helpers.Relax(errMembers5)
		helpers.Relax(errMembers6)
		helpers.Relax(errMembers7)
		helpers.Relax(errDel1)
		helpers.Relax(errDel2)
		helpers.Relax(errDel3)
		helpers.Relax(errDel4)
		helpers.Relax(errDel5)
		helpers.Relax(errDel6)
		helpers.Relax(errDel7)

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
					err = helpers.ElasticUpdateEventLog(
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
					err = helpers.ElasticUpdateEventLog(
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
					err = helpers.ElasticUpdateEventLog(
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
							Value: storeBoolAsString(mentionAbleValue),
						})
						break
					case "hoist":
						hoistValue, _ := change.OldValue.(bool)
						options = append(options, models.ElasticEventlogOption{
							Key:   "role_hoist",
							Value: storeBoolAsString(hoistValue),
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
					err = helpers.ElasticUpdateEventLog(
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
					err = helpers.ElasticUpdateEventLog(
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
					err = helpers.ElasticUpdateEventLog(
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
					err = helpers.ElasticUpdateEventLog(
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
					err = helpers.ElasticUpdateEventLog(
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
					err = helpers.ElasticUpdateEventLog(
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

		elapsed := time.Since(start)
		logger().Infof("did %d audit log backfills, %d entries backfilled, took %s",
			len(channelCreateBackfillGuildIDs)+len(channelDeleteBackfillGuildIDs)+
				len(roleCreateBackfillGuildIDs)+len(roleDeleteBackfillGuildIDs)+
				len(banAddBackfillGuildIDs)+len(banRemoveBackfillGuildIDs)+
				len(memberRemoveBackfillGuildIDs),
			successfulBackfills, elapsed)
		metrics.EventlogAuditLogBackfillTime.Set(elapsed.Seconds())
	}
}

func shouldBackfill(guildID string) (do bool) {
	if helpers.GuildSettingsGetCached(guildID).EventlogDisabled {
		return false
	}

	if helpers.GetMemberPermissions(guildID, cache.GetSession().State.User.ID)&discordgo.PermissionViewAuditLogs != discordgo.PermissionViewAuditLogs {
		return false
	}

	return true
}

func (m *Handler) requestAuditLogBackfill(guildID string, backfillType AuditLogBackfillType) (err error) {
	auditLogBackfillRequestsLock.Lock()
	defer auditLogBackfillRequestsLock.Unlock()

	redis := cache.GetRedisClient()

	switch backfillType {
	case AuditLogBackfillTypeChannelCreate:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "channel create")
		_, err := redis.SAdd(models.AuditLogBackfillTypeChannelCreateRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeChannelDelete:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "channel delete")
		_, err := redis.SAdd(models.AuditLogBackfillTypeChannelDeleteRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeRoleCreate:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "role create")
		_, err := redis.SAdd(models.AuditLogBackfillTypeRoleCreateRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeRoleDelete:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "role delete")
		_, err := redis.SAdd(models.AuditLogBackfillTypeRoleDeleteRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeBanAdd:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "ban add")
		_, err := redis.SAdd(models.AuditLogBackfillTypeBanAddRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeBanRemove:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "ban remove")
		_, err := redis.SAdd(models.AuditLogBackfillTypeBanRemoveRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeMemberRemove:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "member remove")
		_, err := redis.SAdd(models.AuditLogBackfillTypeMemberRemoveRedisSet, guildID).Result()
		return err
	}
	return errors.New("unknown backfill type")
}
