package main

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/sirupsen/logrus"
)

func main() {
	// Read config
	helpers.LoadConfig("config.json")
	config := helpers.GetConfig()

	// setup logger
	log := logrus.New()
	log.Out = os.Stdout
	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.TextFormatter{ForceColors: true, FullTimestamp: true, TimestampFormat: time.RFC3339}
	log.Hooks = make(logrus.LevelHooks)
	cache.SetLogger(log)

	// connect to mongodb
	helpers.ConnectMDB(
		config.Path("mongodb.url").Data().(string),
		config.Path("mongodb.db").Data().(string),
	)
	defer helpers.GetMDbSession().Close()

	// get all idol entries
	var idolEntries []models.BiasGameIdolEntry
	helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(nil)).All(&idolEntries)

	// create results folder
	os.Mkdir("tools/output", 0777)

	// loop through them and download and save
	for _, idolEntry := range idolEntries {
		data, err := helpers.RetrieveFile(idolEntry.ObjectName)
		if err != nil {
			cache.GetLogger().Errorln("error retrieving file", err.Error())
			continue
		}
		err = ioutil.WriteFile("tools/output/"+idolEntry.ObjectName, data, 0644)
		if err != nil {
			cache.GetLogger().Errorln("error writing file", err.Error())
			continue
		}
		cache.GetLogger().Infoln("saved tools/output/" + idolEntry.ObjectName)
	}

	cache.GetLogger().Infoln("done!")
}
