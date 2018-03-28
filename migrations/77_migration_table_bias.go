package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m77_migration_table_bias() {
	if !TableExists("bias") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving bias to mongodb")

	cursor, err := gorethink.Table("bias").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("bias").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	type AssignableRole_Role struct {
		Name      string
		Print     string
		Aliases   []string
		Reactions []string
	}
	type AssignableRole_Category struct {
		Label   string
		Message string
		Pool    string
		Hidden  bool
		Limit   int
		Roles   []AssignableRole_Role
	}
	var rethinkdbEntry struct {
		ID         string                    `gorethink:"id,omitempty"`
		ServerID   string                    `gorethink:"serverid"`
		ChannelID  string                    `gorethink:"channelid"`
		Categories []AssignableRole_Category `gorethink:"categories"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		categories := make([]models.BiasEntryCategory, 0)
		for _, oldCategory := range rethinkdbEntry.Categories {
			roles := make([]models.BiasEntryRole, 0)
			for _, oldRole := range oldCategory.Roles {
				roles = append(roles, models.BiasEntryRole{
					Name:      oldRole.Name,
					Print:     oldRole.Print,
					Aliases:   oldRole.Aliases,
					Reactions: oldRole.Reactions,
				})
			}
			categories = append(categories, models.BiasEntryCategory{
				Label:   oldCategory.Label,
				Message: oldCategory.Message,
				Pool:    oldCategory.Pool,
				Hidden:  oldCategory.Hidden,
				Limit:   oldCategory.Limit,
				Roles:   roles,
			})
		}

		err = helpers.MDbUpsertWithoutLogging(
			models.BiasTable,
			bson.M{"guildid": rethinkdbEntry.ServerID, "channelid": rethinkdbEntry.ChannelID},
			models.BiasEntry{
				GuildID:    rethinkdbEntry.ServerID,
				ChannelID:  rethinkdbEntry.ChannelID,
				Categories: categories,
			},
		)
		if err != nil {
			panic(err)
		}

		bar.Increment()
	}

	if cursor.Err() != nil {
		panic(err)
	}
	bar.Finish()

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb bias")
	_, err = gorethink.TableDrop("bias").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
