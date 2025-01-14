package main

import (
	"github.com/unbasical/doras-server/internal/pkg/utils/logutils"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Parse CLI args.
	var (
		app = kingpin.New("doras-cli", "A command-line tool to work with doras delta patches")

		// Logging
		logLevel  = app.Flag("log-level", "Log-Level, must be one of [DEBUG, INFO, WARN, ERROR]").Default("INFO").Envar("LOG_LEVEL").Enum("DEBUG", "INFO", "WARN", "ERROR", "debug", "info", "warn", "error")
		logFormat = app.Flag("log-format", "Log-Format, must be one of [TEXT, JSON]").Default("TEXT").Envar("LOG_FORMAT").Enum("TEXT", "JSON")
	)
	app.HelpFlag.Short('h')

	// Setup logging.
	setLogLevel(*logLevel)
	setLogFormat(*logFormat)
}

func setLogFormat(logFormat string) {
	switch logFormat {
	case "JSON":
		log.SetFormatter(&logutils.UTCFormatter{Formatter: &log.JSONFormatter{}})
	default:
		log.SetFormatter(&logutils.UTCFormatter{Formatter: &log.TextFormatter{FullTimestamp: true}})
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
