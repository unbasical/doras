package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/aliasing"
	"github.com/unbasical/doras-server/internal/pkg/artifact"
	"github.com/unbasical/doras-server/internal/pkg/storage"
	"github.com/unbasical/doras-server/internal/pkg/utils"
	"io"
	"net/http"
)

func BuildCloudAPI(r *gin.Engine, config *Config) *gin.Engine {
	log.Debug("Building cloud API")

	artifactsAPI := r.Group("/api/artifacts")
	cloudAPI := CloudAPI{
		artifactStorageProvider: config.ArtifactStorage,
		aliasProvider:           config.Aliaser,
	}

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
	artfct, err := shared.readNamedArtifact(identifier)
	if err != nil {
		log.Error(err)
		var inner error
		if errors.Is(err, DorasArtifactNotFoundError) || errors.Is(err, DorasAliasNotFoundError) {
			inner = err
		} else {
			inner = DorasInternalError
		}
		c.JSON(http.StatusNotFound, cloudAPIError{Error: cloudAPIErrorInner{
			Code:    inner,
			Message: fmt.Sprintf("failed to find named artifact for alis %s", identifier)}},
		)
		return
	}
	c.DataFromReader(
		http.StatusOK,
		int64(artfct.GetContentLength()),
		"application/octet-stream",
		artfct.GetReader(),
		// TODO: this should be sanitized or it might allow injecting stuff into the header
		map[string]string{
			"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, identifier),
		},
	)
}

func readArtifact(shared *CloudAPI, c *gin.Context) {
	// assumption: storage.ArtifactStorage and storage.Aliaser handle path sanitization
	identifier := c.Param("identifier")
	artfct, err := shared.readArtifact(identifier)
	if err != nil {
		log.Errorf("Error loading artifact: %v", err)
		if errors.Is(err, DorasArtifactNotFoundError) {
			c.JSON(http.StatusNotFound, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasArtifactNotFoundError}})
		} else {
			c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasInternalError}})
		}
		return
	}
	c.DataFromReader(
		http.StatusOK,
		int64(artfct.GetContentLength()),
		"application/octet-stream",
		artfct.GetReader(),
		// TODO: this should be sanitized or it might allow injecting stuff into the header
		map[string]string{
			"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, identifier),
		},
	)
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
				Message: "artifact not provided in request body",
			},
		})
		return
	}
	identifier, err := shared.createArtifact(&artifact.RawBytesArtifact{Data: data})
	if err != nil {
		log.Errorf("Failed to create artifact %s", err)
		c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasInternalError}})
	}
	c.JSON(http.StatusCreated, gin.H{"success": CreateArtifactResponse{Identifier: identifier}})
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
	identifier, err = shared.createNamedArtifact(&artifact.RawBytesArtifact{Data: data}, alias)
	if err != nil {
		c.JSON(http.StatusInternalServerError, cloudAPIError{Error: cloudAPIErrorInner{Code: DorasInternalError}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": CreateNamedArtifactResponse{
		NamedIdentifier: alias,
		Identifier:      identifier,
	}})
}

// CloudAPI
// TODO:
//   - handle/prevent resource conflicts
//   - replace config with interfaces that provide functionality
//   - move to separate file
type CloudAPI struct {
	artifactStorageProvider storage.ArtifactStorage
	aliasProvider           aliasing.Aliasing
}

func (cloudAPI *CloudAPI) deleteNamedArtifact(identifier string) error {
	panic("not implemented")
}

func (cloudAPI *CloudAPI) deleteArtifact(identifier string) error {
	panic("not implemented")
}

func (cloudAPI *CloudAPI) readAllArtifacts() ([]artifact.Artifact, error) {
	panic("not implemented")
}

func (cloudAPI *CloudAPI) readArtifact(identifier string) (artifact.Artifact, error) {
	artfct, err := cloudAPI.artifactStorageProvider.LoadArtifact(identifier)
	if err != nil {
		log.Errorf("Error loading artifact: %v", err)
		return nil, DorasArtifactNotFoundError
	}
	return artfct, nil
}

func (cloudAPI *CloudAPI) readNamedArtifact(alias string) (artifact.Artifact, error) {
	// resolve the alias to the real identifier
	identifier, err := cloudAPI.aliasProvider.ResolveAlias(alias)
	if err != nil {
		log.Errorf("Error resolving alias: %v", err)
		return nil, DorasAliasNotFoundError
	}
	// now find the artifact using the resolved alias
	artfct, err := cloudAPI.artifactStorageProvider.LoadArtifact(identifier)
	if err != nil {
		log.Errorf("Error loading artifact: %v", err)
		return nil, DorasArtifactNotFoundError
	}
	return artfct, nil
}

func (cloudAPI *CloudAPI) createNamedArtifact(artfct artifact.Artifact, identifier string) (string, error) {
	// improvement idea: use goroutine to parallelize alias creation and artifact storage
	alias := identifier
	// store the artifact at a deterministic location first
	identifier, err := cloudAPI.createArtifact(artfct)
	if err != nil {
		return "", err
	}
	// add an alias to the previously returned identifier
	log.Debugf("adding alias from `%s` -> `%s`", alias, identifier)
	err = cloudAPI.aliasProvider.AddAlias(alias, identifier)
	if err != nil {
		// TODO: add better error handling here to cover different error causes
		log.Errorf("error storing artifact %s", err)
		return "", DorasInternalError
	}
	return identifier, nil
}

func (cloudAPI *CloudAPI) createArtifact(artfct artifact.Artifact) (string, error) {
	// store the artifact at a deterministic location
	data := artfct.GetBytes()
	hash := utils.CalcSha256Hex(data)
	log.Debugf("storing file at %s", hash)
	err := cloudAPI.artifactStorageProvider.StoreArtifact(&artifact.RawBytesArtifact{Data: data}, hash)
	if err != nil {
		// TODO: add better error handling here to cover different error causes
		log.Errorf("error storing artifact %s", err)
		return "", DorasInternalError
	}
	// add an alias to the previously stored artifact
	return hash, nil
}

func extractFile(c *gin.Context, name string) ([]byte, error) {
	formFile, err := c.FormFile(name)
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
