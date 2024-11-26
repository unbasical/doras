package logutils

import "github.com/sirupsen/logrus"

type UTCFormatter struct {
	logrus.Formatter
}

func (u UTCFormatter) Format(e *logrus.Entry) ([]byte, error) {
	e.Time = e.Time.UTC()
	return u.Formatter.Format(e)
}

func SetupTestLogging() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(UTCFormatter{Formatter: &logrus.TextFormatter{FullTimestamp: true}})
}
