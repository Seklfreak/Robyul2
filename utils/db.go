package utils

import (
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/models"
    "time"
    rethink "gopkg.in/gorethink/gorethink.v3"
)

var (
    dbName string
    dbSession *rethink.Session
)

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
    settings := models.Config{}
    err = rethink.Table("config").Get()
   // err = GetDB().C("config").Find(bson.M{"guild" : guild}).One(&settings)

    // Check if the entry is new
    if settings.Data["_"] == "" {
        // Create it
        settings.Data = make(map[string]string)
        settings.Data["_"] = time.Now().String()
        settings.Guild = guild

        GetDB().C("config").Insert(settings)
    }

    settings.Data[key] = value

    //   err = GetDB().C("config").Update(bson.M{"guild" : guild}, settings)

    return err
}

func GuildSettingGet(guild string, key string) (result string, err error) {
    settings := models.Config{}
    //    err = GetDB().C("config").Find(bson.M{"guild" : guild}).One(&settings)
    result = settings.Data[key]

    return result, err
}