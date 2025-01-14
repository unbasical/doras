package core

import (
	"fmt"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/configs"
	"github.com/unbasical/doras-server/internal/pkg/api"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type Doras struct {
	engine   *gin.Engine
	stop     chan bool
	hostname string
	port     uint16
}

// New returns an instance of a Doras server.
func New(config configs.ServerConfig) *Doras {
	doras := Doras{}
	return doras.init(config)
}

// init initializes the
func (d *Doras) init(config configs.ServerConfig) *Doras {
	// TODO: replace repository with a mechanism that resolves a string to a target, e.g. a remote repository
	reg, err := remote.NewRegistry(config.ConfigFile.Storage.URL)
	if err != nil {
		log.Fatalf("failed to create reg for URL: %s, %s", config.ConfigFile.Storage.URL, err)
	}

	clientConfigs := map[string]remote.Client{
		config.ConfigFile.Storage.URL: &auth.Client{},
	}
	reg.PlainHTTP = config.ConfigFile.Storage.EnableHTTP
	d.hostname = config.CliOpts.Host
	d.port = config.CliOpts.HTTPPort

	appConfig := &apicommon.Config{
		ArtifactStorage: apicommon.NewRegistryStorage(reg, ""),
		RepoClients:     clientConfigs,
	}
	if config.CliOpts.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	d.engine = api.BuildApp(appConfig)
	err = d.engine.SetTrustedProxies(config.ConfigFile.TrustedProxies)
	if err != nil {
		log.WithError(err).Fatal("failed to set trusted proxies")
	}

	return d
}

// Start the Doras server.
func (d *Doras) Start() {
	log.Info("Starting doras")
	d.stop = make(chan bool, 1)
	serverURL := fmt.Sprintf("%s:%d", d.hostname, d.port)
	err := d.engine.Run(serverURL)
	if err != nil {
		log.Fatal(err)
	}
}

// Stop the Doras server.
func (d *Doras) Stop() {
	// TODO: use goroutine and channel to handle shutdown
	log.Info("Stopping doras")
	d.stop <- true
	log.Warn("Stop() is not implemented yet")
}
