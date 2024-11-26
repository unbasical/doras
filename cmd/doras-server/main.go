package main

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/unbasical/doras-server/configs"
	"github.com/unbasical/doras-server/internal/pkg/fileutils"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/core"
)

func main() {
	log.SetLevel(log.DebugLevel)
	// TODO: refactor this to allow more configuration via envars
	var (
		configFile = kingpin.Flag("config", "path to config file").ExistingFile()
	)

	kingpin.Parse()
	var config configs.DorasServerConfig
	exists, err := fileutils.SafeReadYAML(*configFile, &config, os.FileMode(0644))
	if !exists || err != nil {
		log.Fatalf("Error reading config file %s: %s", *configFile, err)
	}
	log.Debugf("Config: %+v", config)
	doras := core.New(config)
	doras.Start()
}
