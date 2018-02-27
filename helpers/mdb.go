package helpers

import (
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"

	"reflect"

	"strings"

	"crypto/tls"

	"net"

	"fmt"
	"time"

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

	newID := v.String()
	if newID == "" {
		newID = string(bson.NewObjectId())
		v.SetString(newID)
	}

	err = GetMDb().C(collection.String()).Insert(temp.Interface())

	if err != nil {
		return bson.ObjectId(""), err
	}

	return bson.ObjectId(newID), nil
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

func MdbCollection(collection models.MongoDbCollection) (query *mgo.Collection) {
	return GetMDb().C(collection.String())
}

func MDbIter(query *mgo.Query) (iter *mgo.Iter) {
	start := time.Now()
	iter = query.Iter()
	took := time.Since(start)
	if cache.HasKeen() {
		queryValue := reflect.ValueOf(*query)
		queryOp := queryValue.FieldByName("query").FieldByName("op")

		err := cache.GetKeen().AddEvent("Robyul_MongoDB", &KeenMongoDbEvent{
			Seconds:    took.Seconds(),
			Type:       "query",
			Method:     "MdbIter()",
			Collection: queryOp.FieldByName("collection").String(),
			Query:      fmt.Sprintf("%+v", reflect.ValueOf(queryOp.FieldByName("query")).Interface()),
			Skip:       queryOp.FieldByName("skip").Int(),
			Limit:      queryOp.FieldByName("limit").Int(),
		})
		if err != nil {
			cache.GetLogger().WithField("module", "mdb").Error("Error logging MongoDB request to keen: ", err.Error())
		}
	}
	return
}

func MdbOne(query *mgo.Query, object interface{}) (err error) {
	start := time.Now()
	err = query.One(object)
	took := time.Since(start)
	if cache.HasKeen() {
		queryValue := reflect.ValueOf(*query)
		queryOp := queryValue.FieldByName("query").FieldByName("op")

		err := cache.GetKeen().AddEvent("Robyul_MongoDB", &KeenMongoDbEvent{
			Seconds:    took.Seconds(),
			Type:       "query",
			Method:     "MdbOne()",
			Collection: queryOp.FieldByName("collection").String(),
			Query:      fmt.Sprintf("%+v", reflect.ValueOf(queryOp.FieldByName("query")).Interface()),
			Skip:       queryOp.FieldByName("skip").Int(),
			Limit:      queryOp.FieldByName("limit").Int(),
		})
		if err != nil {
			cache.GetLogger().WithField("module", "mdb").Error("Error logging MongoDB request to keen: ", err.Error())
		}
	}
	return
}

// TODO: add keen logging for the others

type KeenMongoDbEvent struct {
	Seconds    float64
	Type       string
	Method     string
	Collection string
	Query      string
	Skip       int64
	Limit      int64
}
