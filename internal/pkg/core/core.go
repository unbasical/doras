package core

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/configs"
	"github.com/unbasical/doras-server/internal/pkg/api"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type Doras struct {
	engine    *gin.Engine
	stop      chan bool
	serverUrl string
}

func New(config configs.DorasServerConfig) *Doras {
	doras := Doras{}
	return doras.Init(config)
}

func (d *Doras) Init(config configs.DorasServerConfig) *Doras {
	// TODO: replace repository with a mechanism that resolves a string to a target, e.g. a remote repository
	reg, err := remote.NewRegistry(config.Storage.URL)
	if err != nil {
		log.Fatalf("failed to create reg for URL: %s, %s", config.Storage.URL, err)
	}

	clientConfigs := make(map[string]remote.Client, len(config.Sources))
	for k := range config.Sources {
		clientConfigs[k] = &auth.Client{}
	}
	reg.PlainHTTP = config.Storage.EnableHTTP
	if config.Host == "" {
		d.serverUrl = "localhost:8080"
	} else {
		d.serverUrl = config.Host
	}

	appConfig := &apicommon.Config{
		ArtifactStorage: apicommon.NewRegistryStorage(reg, ""),
		RepoClients:     clientConfigs,
	}
	d.engine = api.BuildApp(appConfig)
	return d
}

func (d *Doras) Start() {
	// TODO: use goroutine and channel to handle shutdown
	log.Info("Starting doras")

	d.stop = make(chan bool, 1)
	err := d.engine.Run(d.serverUrl)
	if err != nil {
		log.Fatal(err)
	}
}

func (d *Doras) Stop() {
	// TODO: use goroutine and channel to handle shutdown
	log.Info("Stopping doras")
	d.stop <- true
	log.Warn("Stop() is not implemented yet")
}
