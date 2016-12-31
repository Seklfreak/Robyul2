package utils

import (
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/models"
    rethink "gopkg.in/gorethink/gorethink.v3"
)

var dbSession *rethink.Session

func ConnectDB(url string, db string) {
    Logger.INF("[DB] Connecting to " + url)

    rethink.SetTags("rethink", "json")

    session, err := rethink.Connect(rethink.ConnectOpts{
        Address:  url,
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

func GuildSettingsSet(guild string, config models.Config) error {
    // Check if an config object exists
    var settings models.Config

    cursor, err := rethink.Table("guild_configs").Filter(map[string]interface{}{"guild": guild}).Run(GetDB())
    defer cursor.Close()

    if err != nil {
        return err
    }

    err = cursor.One(&settings)

    switch err {
    // Insert
    case rethink.ErrEmptyResult:
        _, err = rethink.Table("guild_configs").Insert(config).RunWrite(GetDB())
        break

    // Update
    case nil:
        _, err = rethink.Table("guild_configs").Filter(
            map[string]interface{}{"guild": guild},
        ).Update(config).RunWrite(GetDB())
        break

    default:
        panic(err)
    }

    return err
}

func GuildSettingsGet(guild string) (models.Config, error) {
    var settings models.Config
    var cursor *rethink.Cursor
    var err error

    cursor, err = rethink.Table("guild_configs").Filter(map[string]interface{}{"guild": guild}).Run(GetDB())
    defer cursor.Close()

    if err != nil {
        return settings, err
    }

    err = cursor.One(&settings)

    switch err {
    case rethink.ErrEmptyResult:
        settings = models.Config{}.Default(guild)
        return settings, nil
    default:
        return settings, err
    }
}
