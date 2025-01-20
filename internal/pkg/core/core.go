package core

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/configs"
	"github.com/unbasical/doras-server/internal/pkg/api"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/core/dorasengine"
	deltadelegate "github.com/unbasical/doras-server/internal/pkg/delegates/delta"
	registrydelegate "github.com/unbasical/doras-server/internal/pkg/delegates/registry"
	"net/http"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type Doras struct {
	srv      *http.Server
	engine   dorasengine.Engine
	hostname string
	port     uint16
	config   configs.ServerConfig
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
		RepoClients: clientConfigs,
	}
	if config.CliOpts.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	var registryDelegate registrydelegate.RegistryDelegate
	var deltaDelegate deltadelegate.DeltaDelegate
	for repoUrl, repoClient := range appConfig.RepoClients {
		regTarget, err := remote.NewRegistry(repoUrl)
		if err != nil {
			panic(err)
		}
		regTarget.PlainHTTP = true
		regTarget.Client = repoClient
		registryDelegate = registrydelegate.NewRegistryDelegate(repoUrl, regTarget)
		deltaDelegate = deltadelegate.NewDeltaDelegate(repoUrl)
	}
	dorasEngine := dorasengine.NewEngine(registryDelegate, deltaDelegate)
	r := api.BuildApp(dorasEngine)
	err = r.SetTrustedProxies(config.ConfigFile.TrustedProxies)
	if err != nil {
		log.WithError(err).Fatal("failed to set trusted proxies")
	}
	d.srv = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", d.hostname, d.port),
		Handler: r,
	}
	d.engine = dorasEngine
	d.config = config
	return d
}

// Start the Doras server.
func (d *Doras) Start() {
	log.Info("Starting Doras server")

	go func() {
		if err := d.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Fatal("failed to start server")
			panic(err)
		}
	}()
	log.Infof("Listening on %s", d.srv.Addr)
}

// Stop the Doras server.
func (d *Doras) Stop(ctx context.Context) error {
	go d.engine.Stop(ctx)
	return d.srv.Shutdown(ctx)
}
