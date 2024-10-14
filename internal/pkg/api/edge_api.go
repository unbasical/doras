package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/internal/pkg/differ"
	"github.com/unbasical/doras-server/internal/pkg/storage"
	"github.com/unbasical/doras-server/internal/pkg/utils"
	"io"
	"net/http"
)

type EdgeAPI struct {
	artifactStorageProvider storage.ArtifactStorage
	aliasProvider           storage.Aliasing
}

func BuildEdgeAPI(r *gin.Engine, config *Config) *gin.Engine {
	log.Debug("Building edge API")
	shared := &EdgeAPI{
		artifactStorageProvider: config.ArtifactStorage,
		aliasProvider:           config.Aliaser,
	}
	edgeAPI := r.Group("/edge/artifacts")

	edgeAPI.POST("/delta", func(c *gin.Context) {
		createDelta(shared, c)
	})

	edgeAPI.GET("/delta/:identifier", func(c *gin.Context) {
		readDelta(shared, c)
	})

	edgeAPI.GET("/full/:identifier", func(c *gin.Context) {
		readFull(shared, c)
	})
	return r
}

type createDeltaRequestBody struct {
	IdentifierFrom string `json:"identifier_from" binding:"required"`
	IdentifierTo   string `json:"identifier_to" binding:"required"`
	Algorithm      string `json:"algorithm" binding:"required"`
}

type createDeltaResponseBody struct {
	Identifier string `json:"identifier"`
}

func (edgeAPI *EdgeAPI) createDelta(fromIdentifier string, toIdentifier string, algorithm string) (string, error) {
	log.Debug("handling delta creation request")

	var (
		diffAlg differ.Differ
		fileExt string
	)

	switch algorithm {
	case "bsdiff":
		diffAlg, fileExt = differ.Bsdiff{}, ".bsdiff"
	default:
		return "", fmt.Errorf("unsupported diffing algorithm %s", algorithm)
	}
	from, err := edgeAPI.artifactStorageProvider.LoadArtifact(fromIdentifier)
	if err != nil {
		log.Error(err.Error())
		return "", DorasInternalError
	}
	to, err := edgeAPI.artifactStorageProvider.LoadArtifact(toIdentifier)
	if err != nil {
		log.Error(err.Error())
		return "", DorasInternalError
	}
	deltaData := diffAlg.CreateDiff(from, to)
	deltaHash := utils.CalcSha256Hex(deltaData)
	err = edgeAPI.artifactStorageProvider.StoreDelta(
		&delta.RawDiff{Data: deltaData},
		deltaHash+fileExt,
	)
	if err != nil {
		log.Error(err.Error())
		return "", DorasInternalError
	}
	return deltaHash, nil
}

func createDelta(shared *EdgeAPI, c *gin.Context) {
	log.Debug("handling delta creation request")
	var body createDeltaRequestBody
	err := c.BindJSON(&body)
	log.Debug(body)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"code":        DorasMissingRequestBodyError,
			"description": "missing request body",
		}})
		return
	}
	deltaIdentifier, err := shared.createDelta(body.IdentifierFrom, body.IdentifierTo, body.Algorithm)
	if err != nil {
		// TODO: introduce better error handling, e.g. artifacts do not exist, etc.
		log.Error(err.Error())
		c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasInternalError}})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": createDeltaResponseBody{Identifier: deltaIdentifier},
	})
}

func (edgeAPI *EdgeAPI) readDelta(identifier string, algorithm string) (io.Reader, int, error) {
	var fileExt string
	switch algorithm {
	case "bsdiff":
	default:
		fileExt = ".bsdiff"
	}
	deltaData, err := edgeAPI.artifactStorageProvider.LoadDelta(identifier + fileExt)
	if err != nil {
		log.Error(err.Error())
		return nil, 0, DorasDeltaNotFoundError
	}
	reader := deltaData.GetReader()
	contentLength := deltaData.GetContentLen()
	return reader, contentLength, nil
}

func readDelta(shared *EdgeAPI, c *gin.Context) {
	// TODO: handle case where delta is not yet created
	identifier := c.Param("identifier")
	algorithm := c.Query("algorithm")
	reader, contentLength, err := shared.readDelta(identifier, algorithm)
	if err != nil {
		return
	}

	// TODO: this should be sanitized or it might allow injecting stuff into the header
	extraHeaders := map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, identifier),
	}
	c.DataFromReader(http.StatusOK, int64(contentLength), "application/octet-stream", reader, extraHeaders)
}

func (edgeAPI *EdgeAPI) readFull(identifier string) (io.Reader, int, error) {

	deltaData, err := edgeAPI.artifactStorageProvider.LoadArtifact(identifier)
	if err != nil {
		log.Error(err.Error())
		return nil, 0, DorasDeltaNotFoundError
	}
	reader := deltaData.GetReader()
	contentLength := deltaData.GetContentLength()
	return reader, contentLength, nil
}

func readFull(shared *EdgeAPI, c *gin.Context) {
	identifier := c.Param("identifier")
	reader, contentLength, err := shared.readFull(identifier)
	if err != nil {
		return
	}
	// TODO: this should be sanitized or it might allow injecting stuff into the header
	extraHeaders := map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, identifier),
	}
	c.DataFromReader(http.StatusOK, int64(contentLength), "application/octet-stream", reader, extraHeaders)
}
