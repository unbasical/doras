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
	artifactsAPI.POST("/", func(context *gin.Context) {
		createArtifact(&cloudAPI, context)
	})
	artifactsAPI.GET("/", func(c *gin.Context) {
		readAllArtifacts(&cloudAPI, c)
	})
	artifactsAPI.GET("/:identifier", func(c *gin.Context) {
		readArtifact(&cloudAPI, c)
	})
	artifactsAPI.GET("/named/:identifier", func(c *gin.Context) {
		readNamedArtifact(&cloudAPI, c)
	})
	artifactsAPI.DELETE("/named/:identifier", func(c *gin.Context) {
		deleteArtifact(&cloudAPI, c)
	})
	artifactsAPI.DELETE("/:identifier", func(c *gin.Context) {
		deleteNamedArtifact(&cloudAPI, c)
	})
	return r
}

func deleteNamedArtifact(shared *CloudAPI, c *gin.Context) {
	c.JSON(http.StatusNotImplemented, "not implemented")
}

func deleteArtifact(shared *CloudAPI, c *gin.Context) {
	c.JSON(http.StatusNotImplemented, "not implemented")
}

func readAllArtifacts(shared *CloudAPI, c *gin.Context) {
	c.JSON(http.StatusNotImplemented, "not implemented")
}

func readNamedArtifact(shared *CloudAPI, c *gin.Context) {
	// assumption: storage.ArtifactStorage and storage.Aliaser handle path sanitization
	identifier := c.Param("identifier")
	identifier, err := shared.config.Aliaser.ResolveAlias(identifier)
	if err != nil {
		log.Errorf("Failed to resolve alias %s: %s", identifier, err)
		c.JSON(http.StatusNotFound, cloudAPIError{
			Error: cloudAPIErrorInner{
				Code:    DorasAliasNotFoundError,
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
		c.JSON(http.StatusInternalServerError,
			cloudAPIError{Error: cloudAPIErrorInner{Code: DorasArtifactNotFoundError}})
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

type CreateArtifactResponse struct {
	Identifier string `json:"identifier"`
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
		c.JSON(http.StatusBadRequest, cloudAPIError{
			Error: cloudAPIErrorInner{
				Code:    DorasArtifactNotProvidedError,
				Message: "no artifact provided in request body",
			},
		})
		return
	}
	hash := utils.CalcSha256Hex(data)
	err = shared.config.ArtifactStorage.StoreArtifact(artifact.RawBytesArtifact{Data: data}, hash)
	if err != nil {
		log.Errorf("Failed to store artifact %s", err)
		c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasInternalError}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": CreateArtifactResponse{Identifier: hash}})
}

type CreateNamedArtifactResponse struct {
	NamedIdentifier string `json:"named_identifier"`
	Identifier      string `json:"identifier"`
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
		c.JSON(http.StatusBadRequest, cloudAPIError{
			Error: cloudAPIErrorInner{
				Code:    DorasArtifactNotProvidedError,
				Message: "no artifact provided in request body",
			},
		})
		return
	}
	hash := utils.CalcSha256Hex(data)
	log.Debugf("storing file at %s", hash)
	err = shared.config.ArtifactStorage.StoreArtifact(artifact.RawBytesArtifact{Data: data}, hash)
	if err != nil {
		log.Errorf("error storing artifact %s", err)
		c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasInternalError}})
		return
	}
	err = shared.config.Aliaser.AddAlias(alias, hash)
	if err != nil {
		log.Errorf("error adding artifact alias %s", err)
		// TODO: handle cases where the error source is not a name conflict
		c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasAliasExistsError}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": CreateNamedArtifactResponse{
		NamedIdentifier: identifier,
		Identifier:      hash,
	}})
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
