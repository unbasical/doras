package main

import (
	"strings"

	"github.com/alecthomas/kingpin/v2"
	log "github.com/sirupsen/logrus"
)

func main() {
	var (
		app = kingpin.New("doras-cli", "A command-line tool to work with doras delta patches")

		// commands
		//create     = app.Command("create", "Create a delta patch")
		//apply      = app.Command("apply", "Apply a delta patch")
		//fromCreate = create.Flag("from", "path to first file").ExistingFile()
		//toCreate   = create.Flag("to", "path to second file").ExistingFile()
		//outCreate  = create.Flag("out", "first artifact").String()
		//
		//fromApply = apply.Flag("from", "file to apply patch to").ExistingFile()
		//inApply   = apply.Flag("in", "path to patch").ExistingFile()
		//outApply  = apply.Flag("out", "output path").String()
		//algorithm = app.Flag("algorithm", "The algorithm that is used to create the delta patch").Default("bsdiff").Envar("DORAS_ALGORITHM").Enum("bsdiff")
		// Logging
		logLevel  = app.Flag("log-level", "Log-Level, must be one of [DEBUG, INFO, WARN, ERROR]").Default("INFO").Envar("LOG_LEVEL").Enum("DEBUG", "INFO", "WARN", "ERROR", "debug", "info", "warn", "error")
		logFormat = app.Flag("log-format", "Log-Format, must be one of [TEXT, JSON]").Default("TEXT").Envar("LOG_FORMAT").Enum("TEXT", "JSON")
	)
	app.HelpFlag.Short('h')

	setLogLevel(*logLevel)
	setLogFormat(*logFormat)

}

type UTCFormatter struct {
	log.Formatter
}

func (u UTCFormatter) Format(e *log.Entry) ([]byte, error) {
	e.Time = e.Time.UTC()
	return u.Formatter.Format(e)
}
func setLogFormat(logFormat string) {
	switch logFormat {
	case "JSON":
		log.SetFormatter(UTCFormatter{Formatter: &log.JSONFormatter{}})
	default:
		log.SetFormatter(UTCFormatter{Formatter: &log.TextFormatter{FullTimestamp: true}})
	}
}

func setLogLevel(logLevel string) {
	switch strings.ToUpper(logLevel) {
	case "INFO":
		log.SetLevel(log.InfoLevel)
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "WARN":
		log.SetLevel(log.WarnLevel)
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
	}
}
