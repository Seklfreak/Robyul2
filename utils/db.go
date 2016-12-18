package utils

import (
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/models"
    rethink "gopkg.in/gorethink/gorethink.v3"
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

func GuildSettingSet(guild string, key string, value string) error {
    // Create an empty config object
    var settings models.Config

    cursor, err := rethink.Table("guild_configs").Filter(
        rethink.Row.Field("guild").Eq(guild),
    ).Run(GetDB())
    defer cursor.Close()

    if err != nil {
        return err
    }

    err = cursor.One(&settings)

    switch err {
    case rethink.ErrEmptyResult:
        settings = models.Config{
            Data: make(map[string]string),
            Guild: guild,
        }

        settings.Data[key] = value

        _, err = rethink.Table("guild_configs").Insert(settings).RunWrite(GetDB())
        break

    case nil:
        if err != nil {
            return err
        }

        settings.Data[key] = value

        _, err = rethink.Table("guild_configs").Update(settings).RunWrite(GetDB())
        break

    default:
        panic(err)
    }

    return err
}

func GuildSettingGet(guild string, key string) (string, error) {
    var settings models.Config
    var cursor *rethink.Cursor
    var err error

    cursor, err = rethink.Table("guild_configs").Filter(
        rethink.Row.Field("guild").Eq(guild),
    ).Run(GetDB())
    defer cursor.Close()

    if err != nil {
        return "", err
    }

    var marshalled bool
    marshalled, err = cursor.Peek(&settings)
    if !marshalled {
        return "", err
    }

    return settings.Data[key], nil
}
