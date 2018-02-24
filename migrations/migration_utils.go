package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func TableExists(tableName string) (exists bool) {
	cursor, err := rethink.TableList().Run(helpers.GetDB())
	helpers.Relax(err)
	defer cursor.Close()

	var row string
	for cursor.Next(&row) {
		if row == tableName {
			return true
		}
	}

	return false
}

// CreateTableIfNotExists (works like the mysql call)
func CreateTableIfNotExists(tableName string) {
	if !TableExists(tableName) {
		_, err := rethink.TableCreate(tableName).Run(helpers.GetDB())
		helpers.Relax(err)
	}
}

// CreateDBIfNotExists (works like the mysql call)
func CreateDBIfNotExists(dbName string) {
	cursor, err := rethink.DBList().Run(helpers.GetDB())
	helpers.Relax(err)
	defer cursor.Close()

	dbExists := false

	var row string
	for cursor.Next(&row) {
		if row == dbName {
			dbExists = true
			break
		}
	}

	if !dbExists {
		_, err := rethink.DBCreate(dbName).Run(helpers.GetDB())
		helpers.Relax(err)
	}
}
