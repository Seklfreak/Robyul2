package main

import (
    "gopkg.in/mgo.v2"
    Logger "./logger"
    "gopkg.in/mgo.v2/bson"
    "./models"
    "time"
)

var (
    dbName string
    dbSession *mgo.Session
)

func ConnectDB(url string, db string) {
    Logger.INF("[DB] Connecting to " + url)

    dbName = db
    session, err := mgo.Dial(url)

    if err != nil {
        Logger.ERR("[DB] " + err.Error())
        panic(err)
        return
    }

    dbSession = session

    Logger.INF("[DB] Connected!")
}

func GetDB() *mgo.Database {
    return dbSession.DB(dbName)
}

func GuildSettingSet(guild string, key string, value string) (err error) {
    // Create an empty config object
    settings := models.Config{}
    err = GetDB().C("config").Find(bson.M{"guild" : guild}).One(&settings)

    // Check if the entry is new
    if settings.Data["_"] == "" {
        // Create it
        settings.Data = make(map[string]string)
        settings.Data["_"] = time.Now().String()
        settings.Guild = guild

        GetDB().C("config").Insert(settings)
    }

    settings.Data[key] = value

    err = GetDB().C("config").Update(bson.M{"guild" : guild}, settings)

    return err
}

func GuildSettingGet(guild string, key string) (result string, err error) {
    settings := models.Config{}
    err = GetDB().C("config").Find(bson.M{"guild" : guild}).One(&settings)
    result = settings.Data[key]

    return result, err
}