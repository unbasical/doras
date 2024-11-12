package cloudapi

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/artifact"
	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"
	"io"
	"net/http"
	"net/url"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"strings"
)

func BuildCloudAPI(r *gin.Engine, config *apicommon.Config) *gin.Engine {
	log.Debug("Building cloud API")

	artifactsAPI := r.Group("/api/artifacts")
	// TODO: update this initialization
	cloudAPI := CloudAPI{
		storageProvider: config.ArtifactStorage,
		repoClients:     config.RepoClients,
	}

	artifactsAPI.PUT("/named/:identifier", func(context *gin.Context) {
		panic("todo")
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
		panic("todo")
	})
	artifactsAPI.DELETE("/named/:identifier", func(c *gin.Context) {
		deleteArtifact(&cloudAPI, c)
	})
	artifactsAPI.DELETE("/:identifier", func(c *gin.Context) {
		deleteNamedArtifact(&cloudAPI, c)
	})
	return r
}

//nolint:unparam // not yet implemented
func deleteNamedArtifact(shared *CloudAPI, c *gin.Context) {
	c.JSON(http.StatusNotImplemented, "not implemented")
}

//nolint:unparam // not yet implemented
func deleteArtifact(shared *CloudAPI, c *gin.Context) {
	c.JSON(http.StatusNotImplemented, "not implemented")
}

//nolint:unparam // not yet implemented
func readAllArtifacts(shared *CloudAPI, c *gin.Context) {
	c.JSON(http.StatusNotImplemented, "not implemented")
}

func readArtifact(shared *CloudAPI, c *gin.Context) {
	panic("todo")
}

type CreateArtifactResponse struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Tag  string `json:"tag"`
}

// createArtifact
// Stores the artifact provided as a file in the request body.
// TODO:
//   - add option to provide artifact via URL
//   - add option to provide artifact via OCI reference
func createArtifact(shared *CloudAPI, c *gin.Context) {
	from := c.Query("from")
	switch from {
	case "upload":
		data, err := extractFile(c, "artifact")
		if err != nil {
			log.Errorf("Failed to extract artifact %s", err)
			c.JSON(http.StatusBadRequest, dorasErrors.CloudAPIError{
				Error: dorasErrors.CloudAPIErrorInner{
					Code:    dorasErrors.ErrArtifactNotProvided,
					Message: "artifact not provided in request body",
				},
			})
			return
		}
		_, err = shared.createArtifact(&artifact.RawBytesArtifact{Data: data})
		if err != nil {
			log.Errorf("Failed to create artifact %s", err)
			c.JSON(http.StatusInternalServerError, dorasErrors.CloudAPIError{Error: dorasErrors.CloudAPIErrorInner{Code: dorasErrors.ErrInternal}})
		}
		panic("todo")
		c.JSON(http.StatusCreated, gin.H{"success": CreateArtifactResponse{}})
	case "oci":
		var requestBody CreateOCIArtifactRequest
		if err := c.BindJSON(&requestBody); err != nil {
			log.Errorf("Failed to bind request body: %s", err)
			c.JSON(http.StatusInternalServerError, dorasErrors.CloudAPIError{})
			return
		}
		repo, tag, err := parseOciImageString(requestBody.Image)
		if err != nil {
			log.Errorf("Failed to parse OCI image: %s", err)
			c.JSON(http.StatusInternalServerError, dorasErrors.CloudAPIError{})
			return
		}
		src, err := shared.getOrasSource(repo)
		if err != nil {
			log.Errorf("Failed to get oras source: %s", err)
			// unknown source
			c.JSON(http.StatusInternalServerError, dorasErrors.CloudAPIError{})
			return
		}
		dstPath := strings.ReplaceAll(repo, ".", "/")
		dstPath = strings.ReplaceAll(dstPath, ":", "-")
		d, err := shared.createArtifactFromOCIReference(src, dstPath, tag)
		if err != nil {
			log.Errorf("Failed to create artifact from OCI image: %s", err)
			c.JSON(http.StatusInternalServerError, dorasErrors.CloudAPIError{})
			return
		}
		// TODO: return URI here?
		c.JSON(http.StatusCreated, gin.H{"success": CreateArtifactResponse{
			Path: dstPath,
			Tag:  tag,
			Hash: d.Digest.Encoded(),
		}})
	default:
		c.JSON(http.StatusInternalServerError, dorasErrors.CloudAPIError{})
	}
}

// CloudAPI
// TODO:
//   - handle/prevent resource conflicts
//   - replace config with interfaces that provide functionality
//   - move to separate file
type CloudAPI struct {
	storageProvider apicommon.DorasStorage
	repoClients     map[string]remote.Client
}

func (cloudAPI *CloudAPI) getOrasSource(repoUrl string) (oras.ReadOnlyTarget, error) {
	src, err := remote.NewRepository(repoUrl)
	if err != nil {
		panic(err)
	}
	src.PlainHTTP = true
	if c, ok := cloudAPI.repoClients[repoUrl]; ok {
		src.Client = c
	} else {
		log.Debugf("did not find client configuration for %s, using default config", repoUrl)
	}
	return src, nil
}

type CreateOCIArtifactRequest struct {
	Image string `json:"image"`
}

func (cloudAPI *CloudAPI) createArtifactFromOCIReference(src oras.ReadOnlyTarget, base, tag string) (v1.Descriptor, error) {
	ctx := context.Background()

	// TODO: build target reference that does not cause collisions
	s, err := cloudAPI.storageProvider.GetStorage(base)
	if err != nil {
		log.Errorf("Failed to get oras storage: %s", err)
		return v1.Descriptor{}, err
	}
	d, err := oras.Copy(ctx, src, tag, s, tag, oras.DefaultCopyOptions)
	if err != nil {
		log.Errorf("Failed to copy OCI image: %s", err)
		return v1.Descriptor{}, err
	}
	log.Debugf("copied artifact %q to storage", d.Digest)
	return d, nil
}

func (cloudAPI *CloudAPI) createArtifact(artfct artifact.Artifact) (string, error) {
	data := artfct.GetBytes()
	expected := getDescriptor(data)
	s, err := cloudAPI.storageProvider.GetStorage("blobs")
	if err != nil {
		log.Errorf("Failed to get oras storage: %s", err)
		return "", err
	}
	err = s.Push(context.Background(), expected, artfct.GetReader())
	if err != nil {
		return "", err
	}
	return expected.Digest.Encoded(), nil
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

type OciArtifact struct {
	// MediaType is the media type of the object this schema refers to.
	MediaType string `json:"mediaType"`

	// ArtifactType is the IANA media type of the artifact this schema refers to.
	ArtifactType string `json:"artifactType"`

	// Blobs is a collection of blobs referenced by this manifest.
	Blobs []v1.Descriptor `json:"blobs,omitempty"`

	// Subject (reference) is an optional link from the artifact to another manifest forming an association between the artifact and the other manifest.
	Subject *v1.Descriptor `json:"subject,omitempty"`

	// Annotations contains arbitrary metadata for the artifact manifest.
	Annotations map[string]string `json:"annotations,omitempty"`
}

func getDescriptor(data []byte) v1.Descriptor {
	hasher := sha256.New()
	hasher.Write(data)
	descriptor := v1.Descriptor{
		MediaType:    "", // TODO: set media type
		Digest:       digest.NewDigest("sha256", hasher),
		Size:         int64(len(data)),
		URLs:         nil,
		Annotations:  nil, // TODO: add artifact name
		Data:         nil,
		Platform:     nil,
		ArtifactType: "", // TODO: set artifact type
	}
	return descriptor
}

func parseOciImageString(r string) (string, string, error) {
	if !strings.HasPrefix(r, "oci://") {
		r = "oci://" + r
	}
	log.Debugf("Parsing OCI image: %s", r)
	u, err := url.Parse(r)
	if err != nil {
		return "", "", err
	}
	log.Debugf("parsed URL: %s", u)
	split := strings.SplitN(u.Path, ":", 2)

	if len(split) != 2 {
		return "", "", fmt.Errorf("invalid oci image: %s", u.Path)
	}
	return u.Host + split[0], split[1], nil
}