package core

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/configs"
	"github.com/unbasical/doras/internal/pkg/api"
	"github.com/unbasical/doras/internal/pkg/core/dorasengine"
	deltadelegate "github.com/unbasical/doras/internal/pkg/delegates/delta"
	registrydelegate "github.com/unbasical/doras/internal/pkg/delegates/registry"
	"net/http"
	"oras.land/oras-go/v2/registry/remote/credentials"
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
	d.hostname = config.CliOpts.Host
	d.port = config.CliOpts.HTTPPort

	if config.CliOpts.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	credentialStore, err := credentials.NewStore(config.CliOpts.DockerConfigFilePath, credentials.StoreOptions{
		AllowPlaintextPut:        false,
		DetectDefaultNativeStore: true,
	})
	if err != nil {
		log.WithError(err).Fatal("failed to create credential store from docker config file")
		panic(err)
	}
	creds := credentials.Credential(credentialStore)

	registryDelegate := registrydelegate.NewRegistryDelegate(creds, config.CliOpts.InsecureAllowHTTP)
	deltaDelegate := deltadelegate.NewDeltaDelegate()

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
