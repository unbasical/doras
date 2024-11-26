package edgeapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

func SharedStateMiddleware(state *EdgeAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("sharedState", state)
		c.Next()
	}
}

type EdgeAPI struct {
	artifactStorageProvider apicommon.DorasStorage
	repoClients             map[string]remote.Client
}

func BuildEdgeAPI(r *gin.Engine, config *apicommon.Config) *gin.Engine {
	log.Debug("Building edgeapi API")
	shared := &EdgeAPI{
		artifactStorageProvider: config.ArtifactStorage,
	}
	r.Use(SharedStateMiddleware(shared))
	log.Infof("%s", shared)
	edgeAPI := r.Group("/edgeapi/artifacts")

	edgeAPI.GET("/delta", readDelta)

	edgeAPI.GET("/delta/:identifier", func(c *gin.Context) {
		panic("todo")

		// readDelta(shared, c)
	})

	edgeAPI.GET("/full/:identifier", func(c *gin.Context) {
		panic("todo")
		// readFull(shared, c)
	})
	return r
}

// @BasePath /api/v1/delta
// PingExample godoc
// @Summary Request Delta
// @Schemes
// @Description do ping
// @Tags EdgeAPI
// @Accept json
// @Produce json
// @Success 200 {string} {"message":"pong"}
// @Router /api/v1/artifacts/create [get]
// readDelta
// Stores the artifact provided as a file in the request body.
func readDelta(c *gin.Context) {
	var shared *EdgeAPI
	err := apicommon.ExtractStateFromContext(c, &shared)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
	var req apicommon.ReadDeltaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
	from := c.Query("from")
	if from == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing 'from' parameter"})
	}
	to := c.Query("to")
	if to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing 'to' parameter"})
	}

	// TODO: consider parallelizing resolve with channels
	var srcFrom, srcTo oras.ReadOnlyTarget
	var descFrom, descTo v1.Descriptor
	for _, t := range []struct {
		t *oras.ReadOnlyTarget
		i string
		d *v1.Descriptor
	}{{&srcFrom, req.From, &descFrom}, {&srcTo, req.To, &descTo}} {
		repo, tag, err := apicommon.ParseOciImageString(t.i)
		if err != nil {
			log.Errorf("Failed to parse OCI image: %s", err)
			c.JSON(http.StatusInternalServerError, dorasErrors.APIError{})
			return
		}
		src, err := shared.getOrasSource(repo)
		if err != nil {
			log.Errorf("Failed to get oras source: %s", err)
			// unknown source
			c.JSON(http.StatusInternalServerError, dorasErrors.APIError{})
			return
		}
		*t.t = src
		d, err := src.Resolve(c, tag)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dorasErrors.APIError{})
			return
		}
		*t.d = d
	}
	dst, err := shared.artifactStorageProvider.GetStorage("deltas")
	if err != nil {
		c.JSON(http.StatusInternalServerError, dorasErrors.APIError{})
		return
	}

	log.Warnf("currently always using the toImage registry as the source for fetches")
	log.Warn("currently not using the provided accepted algorithms")
	finished := make(chan *v1.Descriptor, 1)
	go func() {
		descDelta, err := delta.CreateDelta(c, srcTo, dst, descFrom, descTo, "")
		if err != nil {
			log.WithError(err).Error("Failed to create delta")
			finished <- nil
			return
		}
		finished <- descDelta
	}()
	desc := <-finished
	if desc == nil {
		c.JSON(http.StatusInternalServerError, dorasErrors.APIError{})
		return
	}
	c.JSON(http.StatusOK, apicommon.ReadDeltaResponse{*desc})
}

func (edgeApi *EdgeAPI) getOrasSource(repoUrl string) (oras.ReadOnlyTarget, error) {
	src, err := remote.NewRepository(repoUrl)
	if err != nil {
		panic(err)
	}
	src.PlainHTTP = true
	if c, ok := edgeApi.repoClients[repoUrl]; ok {
		src.Client = c
	} else {
		log.Debugf("did not find client configuration for %s, using default config", repoUrl)
	}
	return src, nil
}
