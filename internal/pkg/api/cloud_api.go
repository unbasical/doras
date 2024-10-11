package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/artifact"
	"github.com/unbasical/doras-server/internal/pkg/utils"
	"io"
	"net/http"
)

// CloudAPI
// TODO: handle/prevent resource conflicts
type CloudAPI struct {
	config *Config
}

func BuildCloudAPI(r *gin.Engine, config *Config) *gin.Engine {
	log.Debug("Building cloud API")

	artifactsAPI := r.Group("/api/artifacts")
	cloudAPI := CloudAPI{config: config}

	artifactsAPI.PUT("/named/:identifier", func(context *gin.Context) {
		createNamedArtifact(&cloudAPI, context)
	})

	artifactsAPI.POST("", func(context *gin.Context) {
		createArtifact(&cloudAPI, context)
	})

	// List all artifacts
	artifactsAPI.GET("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	// Path to specific artifact
	artifactsAPI.GET(":identifier", func(c *gin.Context) {
		readArtifact(&cloudAPI, c)
	})
	artifactsAPI.GET("/named/:identifier", func(c *gin.Context) {
		readNamedArtifact(&cloudAPI, c)
	})
	artifactsAPI.PUT("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	artifactsAPI.PATCH("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	artifactsAPI.DELETE("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	return r
}

func readNamedArtifact(shared *CloudAPI, c *gin.Context) {
	// assumption: storage.ArtifactStorage and storage.Aliaser handle path sanitization
	identifier := c.Param("identifier")
	identifier, err := shared.config.Aliaser.ResolveAlias(identifier)
	if err != nil {
		log.Errorf("Failed to resolve alias %s: %s", identifier, err)
		c.JSON(http.StatusNotFound, apiError{
			Error: apiErrorInner{
				Code:    "",
				Message: fmt.Sprintf("unknown alias %s", identifier),
			},
		})
		return
	}
	artfct, err := shared.config.ArtifactStorage.LoadArtifact(identifier)
	if err != nil {
		log.Errorf("Error loading artifact: %v", err)
		c.JSON(http.StatusInternalServerError, "internal server error")
		return
	}
	reader := artfct.GetReader()
	contentLength := len(artfct.GetBytes())
	contentType := "application/octet-stream"

	// TODO: this should be sanitized or it might allow injecting stuff into the header
	extraHeaders := map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, identifier),
	}
	c.DataFromReader(http.StatusOK, int64(contentLength), contentType, reader, extraHeaders)
}

func readArtifact(shared *CloudAPI, c *gin.Context) {
	// assumption: storage.ArtifactStorage and storage.Aliaser handle path sanitization
	identifier := c.Param("identifier")
	artfct, err := shared.config.ArtifactStorage.LoadArtifact(identifier)
	if err != nil {
		log.Errorf("Error loading artifact: %v", err)
		c.JSON(http.StatusInternalServerError, "internal server error")
		return
	}
	reader := artfct.GetReader()
	contentLength := len(artfct.GetBytes())
	contentType := "application/octet-stream"

	// TODO: this should be sanitized or it might allow injecting stuff into the header
	extraHeaders := map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, identifier),
	}
	c.DataFromReader(http.StatusOK, int64(contentLength), contentType, reader, extraHeaders)
}

// createArtifact
// Stores the artifact provided as a file in the request body.
// TODO:
//   - add option to provide artifact via URL
//   - add option to provide artifact via OCI reference
func createArtifact(shared *CloudAPI, c *gin.Context) {
	data, err := extractFile(c, "artifact")
	if err != nil {
		log.Errorf("Failed to extract artifact %s", err)
		c.JSON(http.StatusInternalServerError, "internal server error")
	}
	hash := utils.CalcSha256Hex(data)
	err = shared.config.ArtifactStorage.StoreArtifact(artifact.RawBytesArtifact{Data: data}, hash)
	if err != nil {
		log.Errorf("Failed to store artifact %s", err)
		c.JSON(http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"identifier": hash,
	})
}

// createNamedArtifact creates an artifact at this location and set it as the alias.
func createNamedArtifact(shared *CloudAPI, c *gin.Context) {
	// assumption: storage.ArtifactStorage and storage.Aliaser handle path sanitization
	identifier := c.Param("identifier")
	// rename to avoid confusion
	alias := identifier
	data, err := extractFile(c, "artifact")
	if err != nil {
		log.Errorf("Failed to extract artifact %s", err)
		c.JSON(http.StatusInternalServerError, "internal error")
		return
	}
	hash := utils.CalcSha256Hex(data)
	log.Debugf("storing file at %s", hash)
	err = shared.config.ArtifactStorage.StoreArtifact(artifact.RawBytesArtifact{Data: data}, hash)
	if err != nil {
		log.Errorf("error storing artifact %artifactStorage", err)
		c.JSON(http.StatusInternalServerError, "internal error")
		return
	}
	err = shared.config.Aliaser.AddAlias(alias, hash)
	if err != nil {
		log.Errorf("error adding artifact alias %s", err)
		c.JSON(http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(http.StatusOK, "uploaded file")
}

func extractFile(c *gin.Context, name string) ([]byte, error) {
	formFile, err := c.FormFile("artifact")
	if err != nil {
		return nil, err
	}
	file, err := formFile.Open()
	if err != nil {
		return nil, err
	}
	data := make([]byte, formFile.Size)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data[:n], nil
}
