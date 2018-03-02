package helpers

import (
	"time"

	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

func UseruploadsDisableUser(userID, authorID string) (err error) {
	return MDbUpsert(
		models.UseruploadsDisabledUsersTable,
		bson.M{"userid": userID},
		models.UseruploadsDisabledUsersEntry{
			UserID:         userID,
			BannedByUserID: authorID,
			BannedAt:       time.Now(),
		},
	)
}

func UseruploadsIsDisabled(userID string) (disabled bool) {
	var thisDisabledUser models.UseruploadsDisabledUsersEntry
	err := MdbOne(
		MdbCollection(models.UseruploadsDisabledUsersTable).Find(bson.M{"userid": userID}),
		&thisDisabledUser,
	)

	if err == mgo.ErrNotFound {
		return false
	}

	return true
}
