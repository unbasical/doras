package cloudapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/unbasical/doras-server/internal/pkg/funcutils"

	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

// CloudAPI
// TODO:
//   - handle/prevent resource conflicts
//   - replace config with interfaces that provide functionality
//   - move to separate file
type CloudAPI struct {
	storageProvider apicommon.DorasStorage
	repoClients     map[string]remote.Client
}

func BuildCloudAPI(r *gin.Engine, config *apicommon.Config) *gin.Engine {
	log.Debug("Building cloudapi API")
	artifactsApiPath, err := url.JoinPath("/", apicommon.ApiBasePath, "artifacts")
	funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(err), true, "this should never happen")
	artifactsAPI := r.Group(artifactsApiPath)
	// TODO: update this initialization
	cloudAPI := CloudAPI{
		storageProvider: config.ArtifactStorage,
		repoClients:     config.RepoClients,
	}

	artifactsAPI.POST("/", func(context *gin.Context) {
		createArtifact(&cloudAPI, context)
	})

	return r
}

type CreateArtifactResponse struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Tag  string `json:"tag"`
}

// TODO: update this
// @BasePath /api/v1/create
// PingExample godoc
// @Summary ping example
// @Schemes
// @Description do ping
// @Tags CloudAPI
// @Accept json
// @Produce json
// @Success 200 {string} {"message":"pong"}
// @Router /api/v1/artifacts/create [get]
// createArtifact
// Stores the artifact provided as a file in the request body.
// TODO:
//   - add option to provide artifact via URL
func createArtifact(shared *CloudAPI, c *gin.Context) {
	from := c.Query(apicommon.ArtifactSourceParamKey)
	switch from {
	case apicommon.ArtifactSourceParamValueUpload:
		c.JSON(http.StatusNotImplemented, "not implemented")
	case apicommon.ArtifactSourceParamValueOci:
		var requestBody apicommon.CreateOCIArtifactRequest
		if err := c.BindJSON(&requestBody); err != nil {
			log.Errorf("Failed to bind request body: %s", err)
			c.JSON(http.StatusBadRequest, apicommon.APIError{})
			apicommon.RespondWithError(c, http.StatusBadRequest, dorasErrors.ErrUnmarshal, "")
			return
		}
		repo, tag, _, err := apicommon.ParseOciImageString(requestBody.Image)
		if err != nil {
			log.Errorf("Failed to parse OCI image: %s", err)
			apicommon.RespondWithError(c, http.StatusBadRequest, dorasErrors.ErrInvalidOciImage, requestBody.Image)
			return
		}
		src, err := shared.getOrasSource(repo)
		if err != nil {
			log.Errorf("Failed to get oras source: %s", err)
			apicommon.RespondWithError(c, http.StatusInternalServerError, dorasErrors.ErrInternal, "")
			return
		}
		dstPath := strings.ReplaceAll(repo, ".", "/")
		dstPath = strings.ReplaceAll(dstPath, ":", "-")
		d, err := shared.createArtifactFromOCIReference(src, dstPath, tag)
		if err != nil {
			log.Errorf("Failed to create artifact from OCI image: %s", err)
			apicommon.RespondWithError(c, http.StatusInternalServerError, dorasErrors.ErrInternal, "")
			return
		}
		// TODO: evaluate returning URI instead of descriptor here.
		c.JSON(http.StatusCreated, gin.H{"success": CreateArtifactResponse{
			Path: dstPath,
			Tag:  tag,
			Hash: d.Digest.Encoded(),
		}})
	case apicommon.ArtifactSourceParamValueUrl:
		apicommon.RespondWithError(c, http.StatusNotImplemented, dorasErrors.ErrNotYetImplemented, from)
	default:
		apicommon.RespondWithError(c, http.StatusBadRequest, dorasErrors.ErrBadRequest, from)
	}
}

func (cloudAPI *CloudAPI) getOrasSource(repoUrl string) (oras.ReadOnlyTarget, error) {
	src, err := remote.NewRepository(repoUrl)
	if err != nil {
		return nil, err
	}
	src.PlainHTTP = true
	if c, ok := cloudAPI.repoClients[repoUrl]; ok {
		src.Client = c
	} else {
		log.Debugf("did not find client configuration for %s, using default config", repoUrl)
	}
	return src, nil
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
