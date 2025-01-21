package logutils

import (
	"github.com/sirupsen/logrus"
	"strings"
)

// UTCFormatter is a log formatter that prints with UTC timestamps.
type UTCFormatter struct {
	logrus.Formatter
}

// Format the entry with a UTC timestamp.
func (u *UTCFormatter) Format(e *logrus.Entry) ([]byte, error) {
	e.Time = e.Time.UTC()
	return u.Formatter.Format(e)
}

// SetupTestLogging configures the logger to use the debug log level and use a text output formatter.
func SetupTestLogging() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&UTCFormatter{Formatter: &logrus.TextFormatter{FullTimestamp: true}})
}

// SetLogFormat to use either JSON or text based logging.
func SetLogFormat(logFormat string) {
	switch logFormat {
	case "JSON":
		logrus.SetFormatter(&UTCFormatter{Formatter: &logrus.JSONFormatter{}})
	default:
		logrus.SetFormatter(&UTCFormatter{Formatter: &logrus.TextFormatter{FullTimestamp: true}})
	}
}

// SetLogLevel configures the logger to use the provided log level.
// Log level is treated case-insensitive.
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
