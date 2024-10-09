package storage

import (
	"fmt"
	"github.com/unbasical/doras-server/pkg/artifact"
	"os"
	"path/filepath"
)

// ArtifactStorage is an interface that abstracts loading and storing artifacts.
type ArtifactStorage[A artifact.Artifact] interface {
	LoadArtifact(identifier string) (A, error)
	StoreArtifact(artifact A, identifier string) error
}

// FilesystemStorage implements the ArtifactStorage interface.
// It loads and stores artifacts from the file system relative to the specified basePath.
type FilesystemStorage struct {
	basePath string
}

func (s *FilesystemStorage) LoadArtifact(fPath string) (artifact.RawBytesArtifact, error) {
	fPath = filepath.Join(s.basePath, fPath)
	data, err := os.ReadFile(fPath)
	if err != nil {
		return artifact.RawBytesArtifact{}, fmt.Errorf("could not read artifact file from `%s`: %w", fPath, err)
	}
	return artifact.RawBytesArtifact{Data: data}, nil
}

func (s *FilesystemStorage) StoreArtifact(artifact artifact.RawBytesArtifact, fPath string) error {
	fPath = filepath.Join(s.basePath, fPath)
	err := os.WriteFile(fPath, artifact.Data, 0644)
	if err != nil {
		return fmt.Errorf("could not write artifact file to `%s`: %w", fPath, err)
	}
	return nil
}
