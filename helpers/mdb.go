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

type mgoLogger struct {
}

func (mgol mgoLogger) Output(calldepth int, s string) error {
	cache.GetLogger().WithField("module", "mdb").Info(s)
	return nil
}

// ConnectDB connects to mongodb and stores the session
func ConnectMDB(url string, database string) {
	var err error

	log := cache.GetLogger()
	log.WithField("module", "mdb").Info("Connecting to " + url)

	mgoL := new(mgoLogger)
	mgo.SetLogger(mgoL)

	cache.GetLogger()

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

	start := time.Now()
	err = GetMDb().C(collection.String()).Insert(temp.Interface())
	took := time.Since(start)

	if cache.HasKeen() {
		go func() {
			defer Recover()

			err := cache.GetKeen().AddEvent("Robyul_MongoDB", &KeenMongoDbEvent{
				Seconds:    took.Seconds(),
				Type:       "insert",
				Method:     "MDbInsert()",
				Collection: stripRobyulDatabaseFromCollection(collection.String()),
				Data:       truncateKeenValue(fmt.Sprintf("%+v", data)),
			})
			if err != nil {
				cache.GetLogger().WithField("module", "mdb").Error("Error logging MongoDB request to keen: ", err.Error())
			}
		}()
	}

	if err != nil {
		return bson.ObjectId(""), err
	}

	return bson.ObjectId(newID), nil
}

func MDbInsertWithoutLogging(collection models.MongoDbCollection, data interface{}) (rid bson.ObjectId, err error) {
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

func MDbUpdate(collection models.MongoDbCollection, id bson.ObjectId, data interface{}) (err error) {
	if !id.Valid() {
		return errors.New("invalid id")
	}

	start := time.Now()
	err = GetMDb().C(collection.String()).UpdateId(id, data)
	took := time.Since(start)

	if cache.HasKeen() {
		go func() {
			defer Recover()

			err := cache.GetKeen().AddEvent("Robyul_MongoDB", &KeenMongoDbEvent{
				Seconds:    took.Seconds(),
				Type:       "update",
				Method:     "MDbUpdate()",
				Collection: stripRobyulDatabaseFromCollection(collection.String()),
				Id:         MdbIdToHuman(id),
				Data:       truncateKeenValue(fmt.Sprintf("%+v", data)),
			})
			if err != nil {
				cache.GetLogger().WithField("module", "mdb").Error("Error logging MongoDB request to keen: ", err.Error())
			}
		}()
	}

	if err != nil {
		return err
	}

	return nil
}

func MDbUpdateWithoutLogging(collection models.MongoDbCollection, id bson.ObjectId, data interface{}) (rid bson.ObjectId, err error) {
	if !id.Valid() {
		return bson.ObjectId(""), errors.New("invalid id")
	}

	err = GetMDb().C(collection.String()).UpdateId(id, data)

	if err != nil {
		return bson.ObjectId(""), err
	}

	return id, nil
}

func MDbUpsertID(collection models.MongoDbCollection, id bson.ObjectId, data interface{}) (err error) {
	if !id.Valid() {
		return errors.New("invalid id")
	}

	start := time.Now()
	_, err = GetMDb().C(collection.String()).UpsertId(id, data)
	took := time.Since(start)

	if cache.HasKeen() {
		go func() {
			defer Recover()

			err := cache.GetKeen().AddEvent("Robyul_MongoDB", &KeenMongoDbEvent{
				Seconds:    took.Seconds(),
				Type:       "upsert",
				Method:     "MDbUpsertID()",
				Collection: stripRobyulDatabaseFromCollection(collection.String()),
				Id:         MdbIdToHuman(id),
				Data:       truncateKeenValue(fmt.Sprintf("%+v", data)),
			})
			if err != nil {
				cache.GetLogger().WithField("module", "mdb").Error("Error logging MongoDB request to keen: ", err.Error())
			}
		}()
	}

	if err != nil {
		return err
	}

	return nil
}

func MDbUpsert(collection models.MongoDbCollection, selector interface{}, data interface{}) (err error) {
	start := time.Now()
	_, err = GetMDb().C(collection.String()).Upsert(selector, data)
	took := time.Since(start)

	if cache.HasKeen() {
		go func() {
			defer Recover()

			err := cache.GetKeen().AddEvent("Robyul_MongoDB", &KeenMongoDbEvent{
				Seconds:    took.Seconds(),
				Type:       "upsert",
				Method:     "MDbUpsert()",
				Collection: stripRobyulDatabaseFromCollection(collection.String()),
				Query:      fmt.Sprintf("%+v", selector),
				Data:       truncateKeenValue(fmt.Sprintf("%+v", data)),
			})
			if err != nil {
				cache.GetLogger().WithField("module", "mdb").Error("Error logging MongoDB request to keen: ", err.Error())
			}
		}()
	}

	return err
}

func MDbDelete(collection models.MongoDbCollection, id bson.ObjectId) (err error) {
	if !id.Valid() {
		return errors.New("invalid id")
	}

	start := time.Now()
	err = GetMDb().C(collection.String()).RemoveId(id)
	took := time.Since(start)

	if cache.HasKeen() {
		go func() {
			defer Recover()

			err := cache.GetKeen().AddEvent("Robyul_MongoDB", &KeenMongoDbEvent{
				Seconds:    took.Seconds(),
				Type:       "remove",
				Method:     "MDbDelete()",
				Collection: stripRobyulDatabaseFromCollection(collection.String()),
				Id:         MdbIdToHuman(id),
			})
			if err != nil {
				cache.GetLogger().WithField("module", "mdb").Error("Error logging MongoDB request to keen: ", err.Error())
			}
		}()
	}

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
		go func() {
			defer Recover()

			queryValue := reflect.ValueOf(*query)
			queryOp := queryValue.FieldByName("query").FieldByName("op")

			err := cache.GetKeen().AddEvent("Robyul_MongoDB", &KeenMongoDbEvent{
				Seconds:    took.Seconds(),
				Type:       "query",
				Method:     "MdbIter()",
				Collection: stripRobyulDatabaseFromCollection(queryOp.FieldByName("collection").String()),
				Query:      fmt.Sprintf("%+v", reflect.ValueOf(queryOp.FieldByName("query")).Interface()),
				Skip:       queryOp.FieldByName("skip").Int(),
				Limit:      queryOp.FieldByName("limit").Int(),
			})
			if err != nil {
				cache.GetLogger().WithField("module", "mdb").Error("Error logging MongoDB request to keen: ", err.Error())
			}
		}()
	}
	return
}

func MdbOne(query *mgo.Query, object interface{}) (err error) {
	start := time.Now()
	err = query.One(object)
	took := time.Since(start)
	if cache.HasKeen() {
		go func() {
			defer Recover()

			queryValue := reflect.ValueOf(*query)
			queryOp := queryValue.FieldByName("query").FieldByName("op")

			err := cache.GetKeen().AddEvent("Robyul_MongoDB", &KeenMongoDbEvent{
				Seconds:    took.Seconds(),
				Type:       "query",
				Method:     "MdbOne()",
				Collection: stripRobyulDatabaseFromCollection(queryOp.FieldByName("collection").String()),
				Query:      fmt.Sprintf("%+v", reflect.ValueOf(queryOp.FieldByName("query")).Interface()),
				Skip:       queryOp.FieldByName("skip").Int(),
				Limit:      queryOp.FieldByName("limit").Int(),
			})
			if err != nil {
				cache.GetLogger().WithField("module", "mdb").Error("Error logging MongoDB request to keen: ", err.Error())
			}
		}()
	}
	return
}

func MdbOneWithoutLogging(query *mgo.Query, object interface{}) (err error) {
	err = query.One(object)
	return
}

func MDbUpsertWithoutLogging(collection models.MongoDbCollection, selector interface{}, data interface{}) (err error) {
	_, err = GetMDb().C(collection.String()).Upsert(selector, data)
	return err
}

// Returns a human readable ID version of a ObjectID
// id	: the ObjectID to convert
func MdbIdToHuman(id bson.ObjectId) (text string) {
	return fmt.Sprintf(`%x`, string(id))
}

// Returns an ObjectID from a human readable ID
// text	: the human readable ID
func HumanToMdbId(text string) (id bson.ObjectId) {
	return bson.ObjectIdHex(text)
}

func stripRobyulDatabaseFromCollection(input string) (output string) {
	return strings.TrimPrefix(input, mDbDatabase+".")
}

func truncateKeenValue(input string) string {
	if len(input) < 8000 {
		return input
	}
	return input[0:7999]
}

type KeenMongoDbEvent struct {
	Seconds    float64
	Collection string
	Type       string
	Method     string
	Query      string `json:",omitempty"`
	Skip       int64  `json:",omitempty"`
	Limit      int64  `json:",omitempty"`
	Id         string `json:",omitempty"`
	Data       string `json:",omitempty"`
}
