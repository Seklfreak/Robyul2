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

		auditLogBackfillRequestsLock.Lock()
		backfillsToDo := auditLogBackfillRequests
		auditLogBackfillRequests = make([]AuditLogBackfillRequest, 0)
		auditLogBackfillRequestsLock.Unlock()

		var successfulBackfills int

		for _, backfillToDo := range backfillsToDo {
			switch backfillToDo.Type {
			case AuditLogBackfillTypeChannelCreate:
				logger().Infof("doing channel create backfill for guild #%s", backfillToDo.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfillToDo.GuildID, "", "", discordgo.AuditLogActionChannelCreate, 10)
				helpers.RelaxLog(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfillToDo.GuildID, result.TargetID, models.EventlogTypeChannelCreate)
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
				break
			case AuditLogBackfillTypeChannelDelete:
				logger().Infof("doing channel delete backfill for guild #%s", backfillToDo.GuildID)
				results, err := cache.GetSession().GuildAuditLog(backfillToDo.GuildID, "", "", discordgo.AuditLogActionChannelDelete, 10)
				helpers.RelaxLog(err)
				metrics.EventlogAuditLogRequests.Add(1)

				for _, result := range results.AuditLogEntries {
					elasticTime := helpers.GetTimeFromSnowflake(result.ID)

					elasticItems, err := helpers.GetElasticPendingAuditLogBackfillEventlogs(elasticTime, backfillToDo.GuildID, result.TargetID, models.EventlogTypeChannelDelete)
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
				break
			}
			time.Sleep(time.Second * 1)
		}
		elapsed := time.Since(start)
		logger().Infof("did %d audit log backfills, %d entries backfilled, took %s",
			len(backfillsToDo), successfulBackfills, elapsed)
		metrics.EventlogAuditLogBackfillTime.Set(elapsed.Seconds())
	}
}

func (m *Handler) requestAuditLogBackfill(guildID string, backfillType AuditLogBackfillType) {
	// TODO: store pending backfills in redis
	auditLogBackfillRequestsLock.Lock()
	defer auditLogBackfillRequestsLock.Unlock()

	for _, request := range auditLogBackfillRequests {
		if request.GuildID == guildID && request.Type == backfillType {
			return
		}
	}

	auditLogBackfillRequests = append(auditLogBackfillRequests, AuditLogBackfillRequest{
		GuildID: guildID,
		Type:    backfillType,
	})
}
