package helpers

import (
	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"
	"gitlab.com/project-d-collab/dhelpers/mdb"
)

func GetMaxBadgesForGuild(guildID string) (maxBadges int) {
	maxBadges = GuildSettingsGetCached(guildID).LevelsMaxBadges
	if maxBadges == 0 {
		maxBadges = 100
	}
	if maxBadges < 0 {
		maxBadges = 0
	}
	return maxBadges
}

func GetUserUserdata(userID string) (userdata models.ProfileUserdataEntry, err error) {
	err = MdbOne(
		MdbCollection(models.ProfileUserdataTable).Find(bson.M{"userid": userID}),
		&userdata,
	)

	if mdb.ErrNotFound(err) {
		userdata.UserID = userID
		newid, err := MDbInsert(models.ProfileUserdataTable, userdata)
		userdata.ID = newid
		return userdata, err
	}

	return userdata, err
}
