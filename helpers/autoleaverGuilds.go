package helpers

import (
	"errors"
	"fmt"

	"github.com/Seklfreak/Robyul2/models"
	"github.com/davecgh/go-spew/spew"
	"github.com/getsentry/raven-go"
	"github.com/globalsign/mgo"
	"gopkg.in/mgo.v2/bson"
)

func AddAutoleaverGuildID(s string) bool {
	change, err := GetMDb().C(models.AutoleaverStateTable.String()).Upsert(bson.M{"guildid": s}, &models.AutoleaverStateEntry{
		GuildID: s,
	})
	if err != nil {
		raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{})
		return false
	}

	if change.Matched > 0 {
		return false
	}

	return true
}

func RemoveAutoleaverGuildID(s string) bool {
	err := GetMDb().C(models.AutoleaverStateTable.String()).Remove(bson.M{"guildid": s})
	if errors.Is(err, mgo.ErrNotFound) {
		return false
	}
	if err != nil {
		raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{})
		return false
	}

	return true
}
