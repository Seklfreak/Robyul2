package main

import (
	"math/rand"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/migrations"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	cache.SetLogger(log)

	helpers.LoadConfig("../config.json")
	config := helpers.GetConfig()

	helpers.ConnectDB(
		config.Path("rethink.url").Data().(string),
		config.Path("rethink.db").Data().(string),
	)
	defer helpers.GetDB().Close()

	migrations.CreateTableIfNotExists("stats_voicetimes")

	gorethink.Table("stats_voicetimes").IndexCreate("guildid").Run(helpers.GetDB())
	gorethink.Table("stats_voicetimes").IndexCreate("userid").Run(helpers.GetDB())

	var voiceTime struct {
		ID           string    `gorethink:"id,omitempty"`
		GuildID      string    `gorethink:"guildid"`
		ChannelID    string    `gorethink:"channelid"`
		UserID       string    `gorethink:"userid"`
		JoinTimeUtc  time.Time `gorethink:"join_time_utc"`
		LeaveTimeUtc time.Time `gorethink:"leave_time_utc"`
	}

	itemsToCreate := 160000
	guildIDs := []string{"208673735580844032"}
	channelIDs := []string{"267403060043579402", "259640499827245056", "208673735580844033"}
	userIDs := []string{"116620585638821891", "206018367121784832"}
	lengthMin := 60
	lengthMax := 60 * 60 * 8

	rand.Seed(time.Now().Unix())
	nextLeave := time.Now()

	bar := pb.StartNew(itemsToCreate)
	for i := 0; i < itemsToCreate; i++ {
		length := rand.Intn(lengthMax-lengthMin) + lengthMin
		join := nextLeave.Add(-1 * (time.Second * time.Duration(length)))

		voiceTime.GuildID = guildIDs[rand.Intn(len(guildIDs))]
		voiceTime.ChannelID = channelIDs[rand.Intn(len(channelIDs))]
		voiceTime.UserID = userIDs[rand.Intn(len(userIDs))]
		voiceTime.JoinTimeUtc = join
		voiceTime.LeaveTimeUtc = nextLeave
		insert := gorethink.Table("stats_voicetimes").Insert(voiceTime)
		_, err := insert.RunWrite(helpers.GetDB())
		if err != nil {
			panic(err)
		}
		bar.Increment()
		nextLeave = join
	}
	bar.Finish()
}
