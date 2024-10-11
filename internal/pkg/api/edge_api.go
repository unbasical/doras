package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/internal/pkg/differ"
	"github.com/unbasical/doras-server/internal/pkg/utils"
	"net/http"
)

type EdgeAPI struct {
	config *Config
}

func BuildEdgeAPI(r *gin.Engine, config *Config) *gin.Engine {
	log.Debug("Building edge API")
	shared := &EdgeAPI{config: config}
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
	var (
		diffAlg differ.Differ
		fileExt string
	)

	switch body.Algorithm {
	case "bsdiff":
		diffAlg, fileExt = differ.Bsdiff{}, ".bsdiff"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"code":        DorasUnsupportedDiffingAlgorithmError,
			"description": fmt.Sprintf("unsupported diffing algorithm %s", body.Algorithm),
		}})
		return
	}
	from, err := shared.config.ArtifactStorage.LoadArtifact(body.IdentifierFrom)
	if err != nil {
		log.Debug(err.Error())
		c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasInternalError}})
		return
	}
	to, err := shared.config.ArtifactStorage.LoadArtifact(body.IdentifierTo)
	if err != nil {
		log.Debug(err.Error())
		c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasInternalError}})
		return
	}
	deltaData := diffAlg.CreateDiff(from, to)
	deltaHash := utils.CalcSha256Hex(deltaData)
	err = shared.config.ArtifactStorage.StoreDelta(
		&delta.RawDiff{Data: deltaData},
		deltaHash+fileExt,
	)
	if err != nil {
		log.Debug(err.Error())
		c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasInternalError}})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": createDeltaResponseBody{Identifier: deltaHash},
	})
}

func readDelta(shared *EdgeAPI, c *gin.Context) {
	// TODO: handle case where delta is not yet created
	identifier := c.Param("identifier")
	algorithm := c.Query("algorithm")

	var fileExt string
	switch algorithm {
	case "bsdiff":
	default:
		fileExt = ".bsdiff"
	}
	deltaData, err := shared.config.ArtifactStorage.LoadDelta(identifier + fileExt)
	if err != nil {
		log.Debug(err.Error())
		c.JSON(http.StatusNotFound, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasDeltaNotFoundError}})
		return
	}
	reader := deltaData.GetReader()
	contentLength := deltaData.GetContentLen()
	contentType := "application/octet-stream"

	// TODO: this should be sanitized or it might allow injecting stuff into the header
	extraHeaders := map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, identifier),
	}
	c.DataFromReader(http.StatusOK, int64(contentLength), contentType, reader, extraHeaders)
}
func readFull(shared *EdgeAPI, c *gin.Context) {
	c.JSON(http.StatusNotImplemented, "not implemented")
}
