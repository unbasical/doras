package apicommon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

type Config struct {
	ArtifactStorage DorasStorage
	RepoClients     map[string]remote.Client
}

type DorasStorage interface {
	GetStorage(base string) (oras.Target, error)
}

type RegistryStorage struct {
	reg *remote.Registry
}

func (r *RegistryStorage) GetStorage(base string) (oras.Target, error) {
	repo, err := r.reg.Repository(context.Background(), base)
	return repo, err
}

func NewRegistryStorage(reg *remote.Registry) *RegistryStorage {
	return &RegistryStorage{reg: reg}
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

func ParseOciImageString(r string) (string, string, error) {
	if !strings.HasPrefix(r, "oci://") {
		r = "oci://" + r
	}
	logrus.Debugf("Parsing OCI image: %s", r)
	u, err := url.Parse(r)
	if err != nil {
		return "", "", err
	}
	logrus.Debugf("parsed URL: %s", u)
	split := strings.SplitN(u.Path, ":", 2)

	if len(split) != 2 {
		return "", "", fmt.Errorf("invalid oci image: %s", u.Path)
	}
	return u.Host + split[0], split[1], nil
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
