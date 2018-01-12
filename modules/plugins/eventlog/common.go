package eventlog

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/sirupsen/logrus"
)

func logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "eventlog")
}

func storeBoolAsString(input bool) (output string) {
	if input {
		return "yes"
	} else {
		return "no"
	}
}
