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

var CLI struct {
	HTTPPort             uint16 `help:"HTTP port to listen on." default:"8080" env:"DORAS_HTTP_PORT"`
	Host                 string `help:"Hostname to listen on." default:"127.0.0.1" env:"DORAS_HOST"`
	ConfigFilePath       string `help:"Path to the Doras server config file." env:"DORAS_CONFIG_FILE_PATH"`
	DockerConfigFilePath string `help:"Path to the docker config file which is used to access registry credentials." default:"~/.docker/config.json" env:"DOCKER_CONFIG_FILE_PATH"`
	LogLevel             string `help:"Server log level." default:"info" enum:"debug,info,warn,error" env:"DORAS_LOG_LEVEL"`
}

func main() {
	// Parse CLI options
	_ = kong.Parse(&CLI)
	logLevel := StringToLogLevel(CLI.LogLevel)
	log.SetLevel(logLevel)

	var config configs.DorasServerConfig
	exists, err := fileutils.SafeReadYAML(CLI.ConfigFilePath, &config, os.FileMode(0644))
	if !exists || err != nil {
		log.Fatalf("Error reading config file %s: %s", CLI.ConfigFilePath, err)
	}
	log.Debugf("Config: %+v", config)
	doras := core.New(config)
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
