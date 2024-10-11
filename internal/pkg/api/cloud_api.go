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

type CloudAPI struct {
	config *Config
}

func (receiver CloudAPI) CreateOrUpdateArtifact(c *gin.Context) {

}

func BuildCloudAPI(r *gin.Engine, config *Config) *gin.Engine {
	log.Debug("Building cloud API")

	artifactsAPI := r.Group("/api/artifacts")
	cloudAPI := CloudAPI{config: config}

	artifactsAPI.PUT("/named/:identifier", func(context *gin.Context) {
		CreateArtifactWithName(&cloudAPI, context)
	})

	artifactsAPI.POST("", func(context *gin.Context) {
		CreateArtifact(&cloudAPI, context)
	})

	artifactsAPI.POST("/:identifier", func(c *gin.Context) {})
	// List all artifacts
	artifactsAPI.GET("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	// Path to specific artifact
	artifactsAPI.GET(":identifier", func(c *gin.Context) {
		ReadArtifact(&cloudAPI, c)
	})
	artifactsAPI.GET("/named/:identifier", func(c *gin.Context) {
		readerNamedArtifact(&cloudAPI, c)
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
func readerNamedArtifact(shared *CloudAPI, c *gin.Context) {
	identifier := c.Param("identifier")
	// TODO: handle input sanitization
	identifier, err := shared.config.Aliaser.ResolveAlias(identifier)
	if err != nil {
		log.Errorf("Failed to resolve alias %s: %s", identifier, err)
		c.JSON(http.StatusNotFound, "artifact named does not exist")
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

func ReadArtifact(shared *CloudAPI, c *gin.Context) {
	identifier := c.Param("identifier")
	// TODO: handle input sanitization
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

func CreateArtifact(shared *CloudAPI, c *gin.Context) {
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

// CreateArtifactWithName creates an artifact at this location and set it as the alias.
func CreateArtifactWithName(shared *CloudAPI, c *gin.Context) {
	identifier := c.Param("identifier")
	// rename to avoid confusion
	alias := identifier
	data, err := extractFile(c, "artifact")
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
