package gindelegate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/unbasical/doras-server/internal/pkg/auth"
	apidelegate "github.com/unbasical/doras-server/internal/pkg/delegates/api"

	"github.com/gin-gonic/gin"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	error2 "github.com/unbasical/doras-server/internal/pkg/error"
	"github.com/unbasical/doras-server/pkg/constants"
)

// ginDorasContext implements the apidelegate.APIDelegate interface for gin HTTP servers.
type ginDorasContext struct {
	c *gin.Context
}

func (g *ginDorasContext) RequestContext() (context.Context, error) {
	_ = g.c.Request.Context()
	// Overwrite the context because this seems to block requests.
	ctx := context.Background()
	return ctx, nil
}

func (g *ginDorasContext) ExtractClientAuth() (auth.RegistryAuth, error) {
	// Extract the Bearer token from the Auth Header.
	authHeader := g.c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("missing or invalid Authorization header")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	return auth.NewClientAuthFromToken(token), nil
}

// NewDelegate constructs an apidelegate.APIDelegate for a given gin.Context.
func NewDelegate(c *gin.Context) apidelegate.APIDelegate {
	return &ginDorasContext{c: c}
}

func (g *ginDorasContext) HandleError(err error, msg string) {
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
	RespondWithError(g.c, statusCode, err, msg)
}

func (g *ginDorasContext) ExtractParams() (fromImage, toImage string, acceptedAlgorithms []string, err error) {
	// The from image is mandatory, if it does not exist we cannot continue.
	fromImage = g.c.Query(constants.QueryKeyFromDigest)
	if fromImage == "" {
		return "", "", []string{}, error2.ErrMissingQueryParam
	}

	// The to image can either be provided via a tagged image or identified by a digest.
	toTag := g.c.Query(constants.QueryKeyToTag)
	toDigest := g.c.Query(constants.QueryKeyToDigest)

	// Make sure one and only one is provided.
	if (toTag == "" && toDigest == "") || (toTag != "" && toDigest != "") {
		return "", "", []string{}, error2.ErrMissingQueryParam
	}

	// Pick the one that is provided, we already made sure only one of them is set.
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

func (g *ginDorasContext) HandleSuccess(response any) {
	g.c.JSON(http.StatusOK, response)
}

func (g *ginDorasContext) HandleAccepted() {
	g.c.Status(http.StatusAccepted)
}

// RespondWithError sends an error reply to the client.
func RespondWithError(c *gin.Context, statusCode int, err error, errorContext string) {
	c.JSON(statusCode, apicommon.APIError{
		Error: apicommon.APIErrorInner{
			Code:         statusCode,
			Message:      err.Error(),
			ErrorContext: errorContext,
		},
	})
}
