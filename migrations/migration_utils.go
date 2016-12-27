package migrations

import (
    "github.com/sn0w/Karen/helpers"
    "github.com/sn0w/Karen/utils"
    rethink "gopkg.in/gorethink/gorethink.v3"
)

// Create a table if it does not exist
func CreateTableIfNotExists(tableName string) {
    cursor, err := rethink.TableList().Run(utils.GetDB())
    helpers.Relax(err)
    defer cursor.Close()

    tableExists := false

    var row string
    for cursor.Next(&row) {
        if row == tableName {
            tableExists = true
            break
        }
    }

    if !tableExists {
        _, err := rethink.TableCreate(tableName).Run(utils.GetDB())
        helpers.Relax(err)
    }
}

// Create a DB if it does not exist
func CreateDBIfNotExists(dbName string) {
    cursor, err := rethink.DBList().Run(utils.GetDB())
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
        _, err := rethink.DBCreate(dbName).Run(utils.GetDB())
        helpers.Relax(err)
    }
}
