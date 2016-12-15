package utils

import (
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/models"
    rethink "gopkg.in/gorethink/gorethink.v3"
    "errors"
)

var dbSession *rethink.Session

func ConnectDB(url string, db string) {
    Logger.INF("[DB] Connecting to " + url)

    session, err := rethink.Connect(rethink.ConnectOpts{
        Address: url,
        Database: db,
    })

    if err != nil {
        Logger.ERR("[DB] " + err.Error())
        panic(err)
        return
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
    var settings models.Config
    var cursor *rethink.Cursor
    var marshalled bool

    cursor, err = rethink.Table("guild_configs").Filter(map[string]interface{}{"guild":guild}).Run(GetDB())
    marshalled, err = cursor.Peek(&settings)
    if !marshalled {
        err = errors.New("Failed to unmarshal document")
        return
    }

    result = settings.Data[key]
    return
}
