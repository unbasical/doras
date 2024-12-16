package apicommon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

type Config struct {
	ArtifactStorage ArtifactStorage
	RepoClients     map[string]remote.Client
}

type ArtifactStorage interface {
	GetStorage(repoPath string) (string, oras.Target, error)
}

type DeltaStorage interface {
	GetDeltaStorage(from, to v1.Descriptor) (string, oras.Target, error)
}

type RegistryStorage struct {
	baseRepo string
	reg      *remote.Registry
}

func (r *RegistryStorage) GetDeltaStorage(from, to v1.Descriptor) (string, oras.Target, error) {
	repoPath := path.Join(
		from.Digest.Encoded(),
		to.Digest.Encoded(),
	)
	return r.GetStorage(repoPath)
}

func (r *RegistryStorage) GetStorage(repoPath string) (string, oras.Target, error) {
	repoPath = path.Join(r.baseRepo, repoPath)
	repo, err := r.reg.Repository(context.Background(), repoPath)
	return repoPath, repo, err
}

func NewRegistryStorage(reg *remote.Registry, baseRepo string) *RegistryStorage {
	return &RegistryStorage{reg: reg, baseRepo: baseRepo}
}

func ExtractStateFromContext[T any](c *gin.Context, target *T) error {
	state, exists := c.Get("sharedState")
	if !exists {
		return errors.New("shared state not found")
	}
	castedState, ok := state.(T)
	if !ok {
		return errors.New("shared state is not a *T")
	}
	*target = castedState
	return nil
}

const patternOCIImage = `^/([a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(\/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*)((:([a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}))|(@sha256:[a-f0-9]{64}))$`

var regexOCIImage = regexp.MustCompile(patternOCIImage)

func ParseOciImageString(r string) (repoName string, tag string, isDigest bool, err error) {
	if !strings.HasPrefix(r, "oci://") {
		r = "oci://" + r
	}
	logrus.Debugf("Parsing OCI image: %s", r)
	u, err := url.Parse(r)
	if err != nil {
		return "", "", false, err
	}
	matches := regexOCIImage.FindSubmatch([]byte(u.Path))
	if matches == nil {
		return "", "", false, errors.New("invalid OCI image")
	}
	if repoName = string(matches[1]); repoName == "" {
		return "", "", false, errors.New("invalid OCI image")
	}
	if tag = string(matches[9]); tag == "" {
		if tag = string(matches[10]); tag == "" {
			return "", "", false, errors.New("invalid OCI image")
		}
		isDigest = true
	}
	repoName = fmt.Sprintf("%s/%s", u.Host, repoName)
	return
}

func ExtractFile(c *gin.Context, name string) ([]byte, error) {
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

func RespondWithError(c *gin.Context, statusCode int, err error, errorContext string) {
	c.JSON(statusCode, APIError{
		Error: APIErrorInner{
			Code:         statusCode,
			Message:      err.Error(),
			ErrorContext: errorContext,
		},
	})
}

func SharedStateMiddleware[T any](state *T) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("sharedState", state)
		c.Next()
	}
}
