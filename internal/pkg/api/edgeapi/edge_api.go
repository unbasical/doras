package edgeapi

import (
	"net/http"
	"net/url"

	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"

	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/delta"
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
		repoClients:             config.RepoClients,
	}
	r.Use(SharedStateMiddleware(shared))
	log.Infof("%s", shared)
	edgeApiPath, err := url.JoinPath("/", apicommon.ApiBasePath, apicommon.DeltaApiPath)
	if err != nil {
		log.Fatal(err)
	}
	edgeAPI := r.Group(edgeApiPath)
	edgeAPI.GET("/", readDelta)
	return r
}

// TODO: update this
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
		apicommon.RespondWithError(c, http.StatusInternalServerError, dorasErrors.ErrInternal, "")
		return
	}

	from := c.Query("from")
	if from == "" {
		apicommon.RespondWithError(c, http.StatusBadRequest, dorasErrors.ErrMissingQueryParam, "from")
		return
	}
	to := c.Query("to")
	if to == "" {
		apicommon.RespondWithError(c, http.StatusBadRequest, dorasErrors.ErrMissingQueryParam, "to")
		return
	}

	// TODO: consider parallelizing resolve with channels
	var srcFrom, srcTo oras.ReadOnlyTarget
	var descFrom, descTo v1.Descriptor
	for _, t := range []struct {
		t            *oras.ReadOnlyTarget
		i            string
		d            *v1.Descriptor
		mustBeDigest bool
	}{{&srcFrom, from, &descFrom, true}, {&srcTo, to, &descTo, false}} {
		repo, tag, isDigest, err := apicommon.ParseOciImageString(t.i)
		if err != nil {
			log.Errorf("Failed to parse OCI image: %s", err)
			apicommon.RespondWithError(c, http.StatusInternalServerError, dorasErrors.ErrInternal, "")
			return
		}
		if !isDigest && t.mustBeDigest {
			apicommon.RespondWithError(c, http.StatusBadRequest, dorasErrors.ErrBadRequest, "from image must be digest")
			return
		}
		src, err := shared.getOrasSource(repo)
		if err != nil {
			log.Errorf("Failed to get oras source: %s", err)
			apicommon.RespondWithError(c, http.StatusInternalServerError, dorasErrors.ErrInternal, "")
			return
		}
		*t.t = src
		d, err := src.Resolve(c, tag)
		if err != nil {
			apicommon.RespondWithError(c, http.StatusInternalServerError, dorasErrors.ErrInternal, "")
			return
		}
		*t.d = d
	}
	dst, err := shared.artifactStorageProvider.GetStorage("deltas")
	if err != nil {
		apicommon.RespondWithError(c, http.StatusInternalServerError, dorasErrors.ErrInternal, "")
		return
	}
	log.Warnf("currently always using the toImage registry as the source for fetches")
	log.Warn("currently not using the provided accepted algorithms")
	finished := make(chan *v1.Descriptor, 1)
	go func() {
		descDelta, err := delta.CreateDelta(c, srcTo, dst, descFrom, descTo)
		if err != nil {
			log.WithError(err).Error("Failed to create delta")
			finished <- nil
			return
		}
		finished <- descDelta
	}()
	desc := <-finished
	if desc == nil {
		apicommon.RespondWithError(c, http.StatusInternalServerError, dorasErrors.ErrInternal, "")
		return
	}
	c.JSON(http.StatusOK, apicommon.ReadDeltaResponse{Desc: *desc})
}

func (edgeApi *EdgeAPI) getOrasSource(repoUrl string) (oras.ReadOnlyTarget, error) {
	src, err := remote.NewRepository(repoUrl)
	if err != nil {
		return nil, err
	}
	src.PlainHTTP = true
	if c, ok := edgeApi.repoClients[repoUrl]; ok {
		src.Client = c
	} else {
		log.Debugf("did not find client configuration for %s, using default config", repoUrl)
	}
	return src, nil
}
