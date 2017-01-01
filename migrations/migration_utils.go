package migrations

import (
    "github.com/sn0w/Karen/helpers"
    rethink "gopkg.in/gorethink/gorethink.v3"
)

// Create a table if it does not exist
func CreateTableIfNotExists(tableName string) {
    cursor, err := rethink.TableList().Run(helpers.GetDB())
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
        _, err := rethink.TableCreate(tableName).Run(helpers.GetDB())
        helpers.Relax(err)
    }
}

// Create a DB if it does not exist
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
