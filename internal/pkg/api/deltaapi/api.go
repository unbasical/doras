package deltaapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/unbasical/doras-server/pkg/constants"

	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"

	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

func SharedStateMiddleware(state *DeltaAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("sharedState", state)
		c.Next()
	}
}

type DorasContext interface {
	HandleError(err error, msg string)
}

type DeltaAPI struct {
	artifactStorageProvider apicommon.DorasStorage
	repoClients             map[string]remote.Client
}

func BuildEdgeAPI(r *gin.Engine, config *apicommon.Config) *gin.Engine {
	log.Debug("Building edgeapi API")
	shared := &DeltaAPI{
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

// readDelta
// Stores the artifact provided as a file in the request body.
func readDelta(c *gin.Context) {
	dorasContext := GinDorasContext{c: c}
	var shared *DeltaAPI

	err := apicommon.ExtractStateFromContext(c, &shared)
	if err != nil {
		apicommon.RespondWithError(c, http.StatusInternalServerError, dorasErrors.ErrInternal, "")
		return
	}

	from := c.Query(constants.QueryKeyFromDigest)
	if from == "" {
		apicommon.RespondWithError(c, http.StatusBadRequest, dorasErrors.ErrMissingQueryParam, constants.QueryKeyFromDigest)
		return
	}
	to := c.Query(constants.QueryKeyToTag)
	if to == "" {
		apicommon.RespondWithError(c, http.StatusBadRequest, dorasErrors.ErrMissingQueryParam, constants.QueryKeyToTag)
		return
	}
	deltaResponse, err, msg := readDeltaImpl(from, to, shared)
	if err != nil {
		dorasContext.HandleError(err, msg)
	}
	c.JSON(http.StatusOK, deltaResponse)
}

type GinDorasContext struct {
	c *gin.Context
}

func (g *GinDorasContext) HandleError(err error, msg string) {
	var statusCode int
	if errors.Is(err, dorasErrors.ErrAliasNotFound) {
		statusCode = http.StatusNotFound
	}
	if errors.Is(err, dorasErrors.ErrDeltaNotFound) {
		statusCode = http.StatusNotFound
	}
	if errors.Is(err, dorasErrors.ErrArtifactNotFound) {
		statusCode = http.StatusNotFound
	}
	if errors.Is(err, dorasErrors.ErrArtifactNotProvided) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, dorasErrors.ErrInternal) {
		statusCode = http.StatusInternalServerError
	}
	if errors.Is(err, dorasErrors.ErrMissingRequestBody) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, dorasErrors.ErrUnsupportedDiffingAlgorithm) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, dorasErrors.ErrUnmarshal) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, dorasErrors.ErrNotYetImplemented) {
		statusCode = http.StatusNotImplemented
	}
	if errors.Is(err, dorasErrors.ErrBadRequest) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, dorasErrors.ErrInvalidOciImage) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, dorasErrors.ErrMissingQueryParam) {
		statusCode = http.StatusBadRequest
	}
	apicommon.RespondWithError(g.c, statusCode, err, msg)
}

func readDeltaImpl(from string, to string, shared *DeltaAPI) (*apicommon.ReadDeltaResponse, error, string) {
	// Get oras targets and resolve the images into descriptors
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
			return nil, dorasErrors.ErrInternal, ""
		}
		// check for digest to make sure the request is not using a tagged image
		if !isDigest && t.mustBeDigest {
			return nil, dorasErrors.ErrBadRequest, "from image must be digest"
		}
		src, err := shared.getOrasSource(repo)
		if err != nil {
			log.Errorf("Failed to get oras source: %s", err)
			return nil, dorasErrors.ErrInternal, ""
		}
		*t.t = src
		d, err := src.Resolve(context.Background(), tag)
		if err != nil {
			return nil, dorasErrors.ErrInternal, ""
		}
		*t.d = d
	}
	// Get an oras target for where we store the delta
	dst, err := shared.artifactStorageProvider.GetStorage("deltas")
	if err != nil {
		return nil, dorasErrors.ErrInternal, ""
	}
	log.Warnf("currently always using the toImage registry as the source for fetches")
	log.Warn("currently not using the provided accepted algorithms")

	descDelta, err := delta.CreateDelta(context.Background(), srcTo, dst, descFrom, descTo)
	if err != nil {
		return nil, dorasErrors.ErrInternal, "failed to create delta"
	}
	return &apicommon.ReadDeltaResponse{Desc: *descDelta}, nil, ""
}

func (edgeApi *DeltaAPI) getOrasSource(repoUrl string) (oras.ReadOnlyTarget, error) {
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