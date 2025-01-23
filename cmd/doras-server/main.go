package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"time"

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
		log.Errorf("Error reading configFile file %s: %s", serverConfig.CliOpts.ConfigFilePath, err)
	}
	log.Debugf("Config: %+v", configFile)

	// Start up server.
	doras := core.New(serverConfig)
	doras.Start()
	// Wait for an interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// Gracefully shut down the server with a timeout
	gracefulPeriod := time.Duration(serverConfig.CliOpts.ShutdownTimout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), gracefulPeriod)
	log.Infof("shutting down server with a graceful period of %d Seconds ...", gracefulPeriod/time.Second)
	defer cancel()
	if err := doras.Stop(ctx); err != nil {
		log.Fatalf("Server forced to shut down: %s", err)
	}
	log.Println("Server exited gracefully")
}

// StringToLogLevel parse a logrus.Level from the string.
// Converts input to a lowercase string.
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
