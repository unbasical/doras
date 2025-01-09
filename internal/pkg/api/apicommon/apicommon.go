package apicommon

import (
	"context"
	"errors"
	"path"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/gin-gonic/gin"
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
