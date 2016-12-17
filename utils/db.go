package utils

import (
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/models"
    rethink "gopkg.in/gorethink/gorethink.v3"
    "errors"
)

// Holds the db session
var dbSession *rethink.Session

// Connects to the db
// url - The url to connect to
// db  - The db to access
func ConnectDB(url string, db string) {
    Logger.INF("[DB] Connecting to " + url)

    session, err := rethink.Connect(rethink.ConnectOpts{
        Address: url,
        Database: db,
    })

    if err != nil {
        Logger.ERR("[DB] " + err.Error())
        panic(err)
    }

    dbSession = session

    Logger.INF("[DB] Connected!")
}

func GetDB() *rethink.Session {
    return dbSession
}

func GuildSettingSet(guild string, key string, value string) (err error) {
    // Create an empty config object
    var settings models.Config
    var cursor *rethink.Cursor
    var marshalled bool

    cursor, err = rethink.Table("guild_configs").Filter(map[string]interface{}{"guild":guild}).Run(GetDB())

    // Check if there was no result.
    // Cast otherwise
    if err == rethink.ErrEmptyResult {
        settings = models.Config{
            Data: make(map[string]string),
            Guild: guild,
        }

        rethink.Table("guild_configs").Insert(settings).Run(GetDB())
    } else {
        marshalled, err = cursor.Peek(&settings)
        if !marshalled {
            err = errors.New("Failed to unmarshal document")
            return
        }

        settings.Data[key] = value
        _, err = rethink.Table("guild_configs").Update(settings).Run(GetDB())
    }

    return
}

func GuildSettingGet(guild string, key string) (result string, err error) {
    var res interface{}
    res, err = DBGet(
        "guild_configs",
        map[string]interface{}{"guild": guild},
        &models.Config{},
    )

    result = res.(string)

    return
}

// Gets a document from the db
// table  - The table to read from
// filter - The filter to apply
// T      - The type to reflect onto the result
func DBGet(table string, filter map[string]interface{}, T interface{}) (result interface{}, err error) {
    var cursor *rethink.Cursor
    var marshalled bool

    cursor, err = rethink.Table(table).Filter(filter).Run(GetDB())
    marshalled, err = cursor.Peek(&T)

    if !marshalled {
        err = errors.New("Failed to unmarshal document")
        return
    }

    result = T
    return
}

// Inserts $data into the db
// table - The table to wrtie into
// data  - The object to write
func DBInsert(table string, data interface{}) (interface{}, error) {
    return rethink.Table(table).Insert(data).Run(GetDB())
}

// Updates data in the db
// table - The table to change
// data  - The data to write
func DBUpdate(table string, data interface{}) (interface{}, error) {
    return rethink.Table(table).Update(data).Run(GetDB())
}
