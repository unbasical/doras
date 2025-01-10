package main

import (
	"os"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/unbasical/doras-server/internal/pkg/utils/fileutils"

	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/configs"
	"github.com/unbasical/doras-server/internal/pkg/core"
)

func main() {
	serverConfig := configs.ServerConfig{}
	// Parse CLI options
	_ = kong.Parse(&serverConfig.CliOpts)
	logLevel := StringToLogLevel(serverConfig.CliOpts.LogLevel)
	log.SetLevel(logLevel)

	var configFile configs.ServerConfigFile
	exists, err := fileutils.SafeReadYAML(serverConfig.CliOpts.ConfigFilePath, &serverConfig.ConfigFile, os.FileMode(0644))
	if !exists || err != nil {
		log.Fatalf("Error reading configFile file %s: %s", serverConfig.CliOpts.ConfigFilePath, err)
	}
	log.Debugf("Config: %+v", configFile)

	doras := core.New(serverConfig)
	doras.Start()
}

func StringToLogLevel(level string) log.Level {
	switch strings.ToLower(level) {
	case "panic":
		return log.PanicLevel
	case "fatal":
		return log.FatalLevel
	case "error":
		return log.ErrorLevel
	case "warn":
		return log.WarnLevel
	case "info":
		return log.InfoLevel
	case "debug":
		return log.DebugLevel
	case "trace":
		return log.TraceLevel
	default:
		return log.InfoLevel // Default level
	}
}
