package main

import "gopkg.in/mgo.v2"

var (
    dbName string
    dbSession *mgo.Session
)

func ConnectDB(url string, db string) {
    dbName = db
    session, err := mgo.Dial(url)

    if err != nil {
        panic(err)
    }

    dbSession = session

    defer session.Close()
}

func GetDB() *mgo.Database {
    return dbSession.DB(dbName)
}