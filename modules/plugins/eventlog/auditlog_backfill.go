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
		_, errDel1 := redis.Del(models.AuditLogBackfillTypeChannelCreateRedisSet).Result()
		_, errDel2 := redis.Del(models.AuditLogBackfillTypeChannelDeleteRedisSet).Result()
		auditLogBackfillRequestsLock.Unlock()
		helpers.Relax(errMembers1)
		helpers.Relax(errMembers2)
		helpers.Relax(errDel1)
		helpers.Relax(errDel2)

		var successfulBackfills int

		for _, guildID := range channelCreateBackfillGuildIDs {
			if !shouldBackfill(guildID) {
				continue
			}

			logger().Infof("doing channel create backfill for guild #%s", guildID)
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionChannelCreate, 10)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeChannelCreate)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.ElasticAddUserIDToEventLog(
						elasticItems[0].ElasticID,
						result.UserID,
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
			results, err := cache.GetSession().GuildAuditLog(guildID, "", "", discordgo.AuditLogActionChannelDelete, 10)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
					continue
				}
			}
			helpers.Relax(err)
			metrics.EventlogAuditLogRequests.Add(1)

			for _, result := range results.AuditLogEntries {
				elasticTime := helpers.GetTimeFromSnowflake(result.ID)

				elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, guildID, result.TargetID, models.EventlogTypeChannelDelete)
				if err != nil {
					if strings.Contains(err.Error(), "no fitting items found") {
						continue
					}
				}
				helpers.RelaxLog(err)

				if len(elasticItems) >= 1 {
					err = helpers.ElasticAddUserIDToEventLog(
						elasticItems[0].ElasticID,
						result.UserID,
						true,
					)
					helpers.RelaxLog(err)
					successfulBackfills++
				}
			}
		}

		elapsed := time.Since(start)
		logger().Infof("did %d audit log backfills, %d entries backfilled, took %s",
			len(channelCreateBackfillGuildIDs)+len(channelDeleteBackfillGuildIDs), successfulBackfills, elapsed)
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
		_, err := redis.SAdd(models.AuditLogBackfillTypeChannelCreateRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeChannelDelete:
		_, err := redis.SAdd(models.AuditLogBackfillTypeChannelDeleteRedisSet, guildID).Result()
		return err
	}
	return errors.New("unknown backfill type")
}
