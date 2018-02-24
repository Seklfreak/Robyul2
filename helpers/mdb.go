package helpers

import (
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"reflect"

	"strings"

	"crypto/tls"

	"net"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/pkg/errors"
)

var (
	mDbSession  *mgo.Session
	mDbDatabase string
)

// ConnectDB connects to mongodb and stores the session
func ConnectMDB(url string, database string) {
	var err error

	log := cache.GetLogger()
	log.WithField("module", "mdb").Info("Connecting to " + url)

	// TODO: logger
	//mgo.SetLogger(cache.GetLogger())

	newUrl := strings.TrimSuffix(url, "?ssl=true")
	newUrl = strings.Replace(newUrl, "ssl=true&", "", -1)

	dialInfo, err := mgo.ParseURL(newUrl)
	if err != nil {
		log.WithField("module", "mdb").Error(err.Error())
		panic(err)
	}

	// setup TLS if we use SSL
	if newUrl != url {
		tlsConfig := &tls.Config{}
		tlsConfig.InsecureSkipVerify = true

		dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
			return conn, err
		}
	}

	mDbSession, err = mgo.DialWithInfo(dialInfo)
	if err != nil {
		log.WithField("module", "mdb").Error(err.Error())
		panic(err)
	}

	mDbSession.SetMode(mgo.Monotonic, true)
	mDbSession.SetSafe(&mgo.Safe{WMode: "majority"})

	mDbDatabase = database

	log.WithField("module", "mdb").Info("Connected!")
}

// GetDB is a simple getter for the mongodb database.
func GetMDb() *mgo.Database {
	return mDbSession.DB(mDbDatabase)
}

// GetDB is a simple getter for the mongodb session.
func GetMDbSession() *mgo.Session {
	return mDbSession
}

func MDbInsert(collection models.MongoDbCollection, data interface{}) (rid bson.ObjectId, err error) {
	ptr := reflect.New(reflect.TypeOf(data))
	temp := ptr.Elem()
	temp.Set(reflect.ValueOf(data))

	v := temp.FieldByName("ID")

	if !v.IsValid() {
		return bson.ObjectId(""), errors.New("invalid data")
	}

	newID := bson.NewObjectId()
	if v.String() == "" {
		v.SetString(reflect.ValueOf(newID).String())
	}

	err = GetMDb().C(collection.String()).Insert(temp.Interface())

	if err != nil {
		return bson.ObjectId(""), err
	}

	return newID, nil
}

func MDbUpdate(collection models.MongoDbCollection, id bson.ObjectId, data interface{}) (rid bson.ObjectId, err error) {
	if !id.Valid() {
		return bson.ObjectId(""), errors.New("invalid id")
	}

	err = GetMDb().C(collection.String()).UpdateId(id, data)

	if err != nil {
		return bson.ObjectId(""), err
	}

	return id, nil
}

func MDbUpsertID(collection models.MongoDbCollection, id bson.ObjectId, data interface{}) (rid bson.ObjectId, err error) {
	if !id.Valid() {
		id = bson.NewObjectId()
	}

	_, err = GetMDb().C(collection.String()).UpsertId(id, data)

	if err != nil {
		return bson.ObjectId(""), err
	}

	return id, nil
}

func MDbUpsert(collection models.MongoDbCollection, selector interface{}, data interface{}) (err error) {
	_, err = GetMDb().C(collection.String()).Upsert(selector, data)

	return err
}

func MDbDelete(collection models.MongoDbCollection, id bson.ObjectId) (err error) {
	if !id.Valid() {
		return errors.New("invalid id")
	}

	err = GetMDb().C(collection.String()).RemoveId(id)

	if err != nil {
		return err
	}

	return nil
}

func MDbFind(collection models.MongoDbCollection, selection interface{}) (query *mgo.Query) {
	return GetMDb().C(collection.String()).Find(selection)
}
