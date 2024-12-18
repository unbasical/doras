package deltaapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	error2 "github.com/unbasical/doras-server/internal/pkg/error"
	"github.com/unbasical/doras-server/pkg/constants"
)

type APIDelegate interface {
	ExtractParams() (fromImage, toImage string, acceptedAlgorithms []string, err error)
	HandleError(err error, msg string)
	HandleSuccess(apicommon.ReadDeltaResponse)
	HandleAccepted()
}

type GinDorasContext struct {
	c *gin.Context
}

func (g *GinDorasContext) HandleError(err error, msg string) {
	var statusCode int
	if errors.Is(err, error2.ErrAliasNotFound) {
		statusCode = http.StatusNotFound
	}
	if errors.Is(err, error2.ErrDeltaNotFound) {
		statusCode = http.StatusNotFound
	}
	if errors.Is(err, error2.ErrArtifactNotFound) {
		statusCode = http.StatusNotFound
	}
	if errors.Is(err, error2.ErrArtifactNotProvided) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, error2.ErrInternal) {
		statusCode = http.StatusInternalServerError
	}
	if errors.Is(err, error2.ErrMissingRequestBody) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, error2.ErrUnsupportedDiffingAlgorithm) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, error2.ErrUnmarshal) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, error2.ErrNotYetImplemented) {
		statusCode = http.StatusNotImplemented
	}
	if errors.Is(err, error2.ErrBadRequest) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, error2.ErrInvalidOciImage) {
		statusCode = http.StatusBadRequest
	}
	if errors.Is(err, error2.ErrMissingQueryParam) {
		statusCode = http.StatusBadRequest
	}
	apicommon.RespondWithError(g.c, statusCode, err, msg)
}

func (g *GinDorasContext) ExtractParams() (fromImage, toImage string, acceptedAlgorithms []string, err error) {
	fromImage = g.c.Query(constants.QueryKeyFromDigest)
	if fromImage == "" {
		return "", "", []string{}, error2.ErrMissingQueryParam
	}
	toTag := g.c.Query(constants.QueryKeyToTag)
	toDigest := g.c.Query(constants.QueryKeyToDigest)
	if (toTag == "" && toDigest == "") || (toTag != "" && toDigest != "") {
		return "", "", []string{}, error2.ErrMissingQueryParam
	}
	if toTag != "" {
		toImage = toTag
	}
	if toDigest != "" {
		toImage = toDigest
	}
	acceptedAlgorithms = g.c.QueryArray("acceptedAlgorithms")
	if len(acceptedAlgorithms) == 0 {
		acceptedAlgorithms = constants.DefaultAlgorithms()
	}
	return fromImage, toImage, acceptedAlgorithms, nil
}

func (g *GinDorasContext) HandleSuccess(deltaResponse apicommon.ReadDeltaResponse) {
	g.c.JSON(http.StatusOK, deltaResponse)
}

func (g *GinDorasContext) HandleAccepted() {
	g.c.Status(http.StatusAccepted)
}
