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
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"net/http"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"os"
	"time"
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
	var opts []func(aggregate *ociutils.CredFuncAggregate)

	// Load credentials configured in the config file.
	for regName, regConf := range config.ConfigFile.Registries {
		authConf := regConf.Auth
		if (authConf.Username == "" || authConf.Password == "") && authConf.AccessToken == "" {
			log.Warnf("config file provided no credenitals for registry %s", regName)
		}
		if authConf.AccessToken != "" {
			token := os.ExpandEnv(authConf.AccessToken)
			opts = append(opts, ociutils.WithCredFunc(auth.StaticCredential(regName, auth.Credential{AccessToken: token})))
		}
		if authConf.Password != "" && authConf.Username != "" {
			username := os.ExpandEnv(authConf.Username)
			password := os.ExpandEnv(authConf.Password)
			opts = append(opts, ociutils.WithCredFunc(auth.StaticCredential(regName, auth.Credential{Username: username, Password: password})))
		}
	}
	// Load credentials configured via the docker config file.
	// This is done second so it does not shadow credentials loaded from the config file.
	if config.CliOpts.DockerConfigFilePath != "" {
		credentialStore, err := credentials.NewStore(config.CliOpts.DockerConfigFilePath, credentials.StoreOptions{
			AllowPlaintextPut:        false,
			DetectDefaultNativeStore: true,
		})
		if err != nil {
			log.WithError(err).Fatal("failed to create credential store from docker config file")
		}
		opts = append(opts, ociutils.WithCredFunc(credentials.Credential(credentialStore)))
	}
	creds := ociutils.NewCredentialsAggregate(opts...)

	registryDelegate := registrydelegate.NewRegistryDelegate(creds, config.CliOpts.InsecureAllowHTTP)
	deltaDelegate := deltadelegate.NewDeltaDelegate(time.Duration(config.CliOpts.DummyExpirationDurationMins) * time.Minute)

	dorasEngine := dorasengine.NewEngine(registryDelegate, deltaDelegate, config.CliOpts.RequireClientAuth)
	r := api.BuildApp(dorasEngine)
	err := r.SetTrustedProxies(config.ConfigFile.TrustedProxies)
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
