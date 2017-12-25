package migrations

import (
	"context"

	"time"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/gorethink/gorethink"
)

func m53_move_rethinkdb_voicesessions_to_elasticsearch() {
	if !cache.HasElastic() {
		return
	}

	elastic := cache.GetElastic()
	exists, err := elastic.IndexExists("robyul-voice_session").Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !exists {
		return
	}

	cursor, err := gorethink.TableList().Run(helpers.GetDB())
	helpers.Relax(err)
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

	res, err := gorethink.Table("stats_voicetimes").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}

	var voiceTime struct {
		ID           string    `gorethink:"id,omitempty"`
		GuildID      string    `gorethink:"guildid"`
		ChannelID    string    `gorethink:"channelid"`
		UserID       string    `gorethink:"userid"`
		JoinTimeUtc  time.Time `gorethink:"join_time_utc"`
		LeaveTimeUtc time.Time `gorethink:"leave_time_utc"`
	}

	for res.Next(&voiceTime) {
		err = helpers.ElasticAddVoiceSession(voiceTime.GuildID, voiceTime.ChannelID, voiceTime.UserID,
			voiceTime.JoinTimeUtc, voiceTime.LeaveTimeUtc)
		if err != nil {
			panic(err)
		}
		fmt.Print(".")
	}
	fmt.Println()
	if res.Err() != nil {
		panic(err)
	}

	_, err = gorethink.TableDrop("stats_voicetimes").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
