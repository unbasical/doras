package api

import (
	"github.com/unbasical/doras-server/internal/pkg/core/deltaengine"
	"github.com/unbasical/doras-server/internal/pkg/delegates/api/gindelegate"
	"github.com/unbasical/doras-server/internal/pkg/delegates/delta"
	"github.com/unbasical/doras-server/internal/pkg/delegates/registry"
	"net/http"
	"net/url"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
)

func BuildApp(config *apicommon.Config) *gin.Engine {
	log.Debug("Building app")
	r := gin.Default()
	r = BuildEdgeAPI(r, config)
	r.GET("/api/v1/ping", ping)

	return r
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

func BuildEdgeAPI(r *gin.Engine, config *apicommon.Config) *gin.Engine {
	log.Debug("Building edgeapi API")

	var reg registrydelegate.RegistryDelegate
	var deltaDelegate deltadelegate.DeltaDelegate
	for repoUrl, repoClient := range config.RepoClients {
		regTarget, err := remote.NewRegistry(repoUrl)
		if err != nil {
			panic(err)
		}
		regTarget.PlainHTTP = true
		regTarget.Client = repoClient
		reg = registrydelegate.NewRegistryDelegate(repoUrl, regTarget)
		deltaDelegate = deltadelegate.NewDeltaDelegate(repoUrl)
	}
	deltaEngine := deltaengine.NewDeltaEngine(reg, deltaDelegate)

	edgeApiPath, err := url.JoinPath("/", apicommon.ApiBasePath, apicommon.DeltaApiPath)
	if err != nil {
		log.Fatal(err)
	}
	edgeAPI := r.Group(edgeApiPath)
	edgeAPI.GET("/", func(c *gin.Context) {
		apiDelegate := gindelegate.NewDelegate(c)
		deltaEngine.HandleReadDelta(apiDelegate)
	})
	return r
}
