package apicommon

import (
	"context"
	"path"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

type Config struct {
	RepoClients map[string]remote.Client
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
