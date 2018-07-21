package logging

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

// https://github.com/sirupsen/logrus/issues/230#issuecomment-323387380

type LogrusFileHook struct {
	file      *os.File
	formatter *logrus.JSONFormatter
}

func NewLogrusFileHook(file string, flag int, chmod os.FileMode) (*LogrusFileHook, error) {
	jsonFormatter := &logrus.JSONFormatter{}
	logFile, err := os.OpenFile(file, flag, chmod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook %v", err)
		return nil, err
	}

	return &LogrusFileHook{logFile, jsonFormatter}, err
}

// Fire event
func (hook *LogrusFileHook) Fire(entry *logrus.Entry) error {

	plainformat, err := hook.formatter.Format(entry)
	line := string(plainformat)
	_, err = hook.file.WriteString(line)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook(entry.String)%v", err)
		return err
	}

	return nil
}

func (hook *LogrusFileHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}
