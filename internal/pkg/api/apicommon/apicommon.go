package apicommon

import (
	"context"
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
