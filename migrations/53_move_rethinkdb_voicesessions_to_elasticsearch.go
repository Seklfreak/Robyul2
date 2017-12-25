package migrations

import (
	"context"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
	"github.com/olivere/elastic"
)

func m53_move_rethinkdb_voicesessions_to_elasticsearch() {
	if !cache.HasElastic() {
		return
	}

	elasticClient := cache.GetElastic()
	exists, err := elasticClient.IndexExists("robyul-voice_session").Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !exists {
		return
	}

	cursor, err := gorethink.TableList().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	tableExists := false

	var row string
	for cursor.Next(&row) {
		if row == "stats_voicetimes" {
			tableExists = true
			break
		}
	}

	if !tableExists {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving rethinkdb voice session to elasticsearch")

	cursor, err = gorethink.Table("stats_voicetimes").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("stats_voicetimes").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var voiceTime struct {
		ID           string    `gorethink:"id,omitempty"`
		GuildID      string    `gorethink:"guildid"`
		ChannelID    string    `gorethink:"channelid"`
		UserID       string    `gorethink:"userid"`
		JoinTimeUtc  time.Time `gorethink:"join_time_utc"`
		LeaveTimeUtc time.Time `gorethink:"leave_time_utc"`
	}

	bulkProcessor, err := elasticClient.BulkProcessor().
		Name("rethinkdb-voice-sessions-migration-worker").Workers(4).Do(context.Background())
	if err != nil {
		panic(err)
	}
	defer bulkProcessor.Close()

	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&voiceTime) {
		duration := voiceTime.LeaveTimeUtc.Sub(voiceTime.JoinTimeUtc)

		elasticVoiceSessionData := models.ElasticVoiceSession{
			CreatedAt:       voiceTime.LeaveTimeUtc,
			GuildID:         voiceTime.GuildID,
			ChannelID:       voiceTime.ChannelID,
			UserID:          voiceTime.UserID,
			JoinTime:        voiceTime.JoinTimeUtc,
			LeaveTime:       voiceTime.LeaveTimeUtc,
			DurationSeconds: int64(duration.Seconds()),
		}

		request := elastic.NewBulkIndexRequest().
			Index(models.ElasticIndexVoiceSessions).
			Type("doc").Doc(elasticVoiceSessionData)
		bulkProcessor.Add(request)

		bar.Increment()
	}
	bulkProcessor.Flush()

	if cursor.Err() != nil {
		panic(err)
	}
	bar.Finish()

	_, err = gorethink.TableDrop("stats_voicetimes").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
