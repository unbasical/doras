package deltaapi

import (
	"net/url"

	"github.com/unbasical/doras-server/internal/pkg/api/deltaengine"

	"github.com/unbasical/doras-server/internal/pkg/api/deltadelegate"

	"oras.land/oras-go/v2/registry/remote"

	"github.com/unbasical/doras-server/internal/pkg/api/registrydelegate"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
)

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
		dorasContext := GinDorasContext{c: c}
		deltaEngine.HandleReadDelta(&dorasContext)
	})
	return r
}
