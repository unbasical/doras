package main

import (
	"context"
	"fmt"
	"github.com/unbasical/doras/common"
	"github.com/unbasical/doras/examples"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/alecthomas/kong"

	"github.com/unbasical/doras/internal/pkg/utils/fileutils"

	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/configs"
	"github.com/unbasical/doras/internal/pkg/core"
)

func main() {
	serverConfig := configs.ServerConfig{}
	// Parse CLI options
	kongCtx := kong.Parse(&serverConfig.CliOpts)
	if serverConfig.CliOpts.Version {
		println(common.Version())
		return
	}
	logLevel := StringToLogLevel(serverConfig.CliOpts.LogLevel)
	log.SetLevel(logLevel)

	if kongCtx.Command() == "example-config" {
		exampleConfig := examples.DorasExampleConfig()
		if serverConfig.CliOpts.ExampleConfig.Output != "" {
			file, err := os.OpenFile(serverConfig.CliOpts.ExampleConfig.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				log.Fatal(err)
			}
			_, err = file.Write([]byte(exampleConfig))
			if err != nil {
				log.Fatal(err)
			}
			_ = file.Close()
			log.Infof("Example config written to %s", serverConfig.CliOpts.ExampleConfig.Output)
			return
		}
		_, _ = fmt.Printf("%s\n", exampleConfig)
		return
	}

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
