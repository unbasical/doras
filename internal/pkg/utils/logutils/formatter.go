package logutils

import (
	"github.com/sirupsen/logrus"
	"strings"
)

// UTCFormatter is a log formatter that prints with UTC timestamps.
type UTCFormatter struct {
	logrus.Formatter
}

func (u *UTCFormatter) Format(e *logrus.Entry) ([]byte, error) {
	e.Time = e.Time.UTC()
	return u.Formatter.Format(e)
}

func SetupTestLogging() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&UTCFormatter{Formatter: &logrus.TextFormatter{FullTimestamp: true}})
}

func SetLogFormat(logFormat string) {
	switch logFormat {
	case "JSON":
		logrus.SetFormatter(&UTCFormatter{Formatter: &logrus.JSONFormatter{}})
	default:
		logrus.SetFormatter(&UTCFormatter{Formatter: &logrus.TextFormatter{FullTimestamp: true}})
	}
}

func SetLogLevel(logLevel string) {
	switch strings.ToUpper(logLevel) {
	case "INFO":
		logrus.SetLevel(logrus.InfoLevel)
	case "DEBUG":
		logrus.SetLevel(logrus.DebugLevel)
	case "WARN":
		logrus.SetLevel(logrus.WarnLevel)
	case "ERROR":
		logrus.SetLevel(logrus.ErrorLevel)
	}
}
