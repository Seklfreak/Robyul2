package helpers

import (
    Logger "git.lukas.moe/sn0w/Karen/logger"
    "git.lukas.moe/sn0w/Karen/models"
    rethink "gopkg.in/gorethink/gorethink.v3"
)

var dbSession *rethink.Session

// ConnectDB connects to rethink and stores the session
func ConnectDB(url string, db string) {
    Logger.INFO.L("db", "Connecting to " + url)

    rethink.SetTags("rethink", "json")

    session, err := rethink.Connect(rethink.ConnectOpts{
        Address:  url,
        Database: db,
    })

    if err != nil {
        Logger.ERROR.L("db", err.Error())
        panic(err)
    }

    dbSession = session

    Logger.INFO.L("db", "Connected!")
}

// GetDB is a simple getter for the rethink session.
// Might receive some singleton-like lazy-creation later
func GetDB() *rethink.Session {
    return dbSession
}

// GuildSettingsSet writes all $config into the db
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

// GuildSettingsGet returns all config values for the guild or a default object
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

// GetPrefixForServer gets the prefix for $guild
func GetPrefixForServer(guild string) (string, error) {
    settings, err := GuildSettingsGet(guild)
    if err != nil {
        return "", err
    }

    return settings.Prefix, nil
}

// SetPrefixForServer sets the prefix for $guild to $prefix
func SetPrefixForServer(guild string, prefix string) error {
    settings, err := GuildSettingsGet(guild)
    if err != nil {
        return err
    }

    settings.Prefix = prefix

    return GuildSettingsSet(guild, settings)
}
