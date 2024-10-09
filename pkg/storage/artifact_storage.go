package storage

import (
	"fmt"
	"github.com/unbasical/doras-server/pkg/artifact"
	"github.com/unbasical/doras-server/pkg/delta"
	"io"
	"os"
	"path/filepath"
)

// ArtifactStorage is an interface that abstracts loading and storing artifacts.
type ArtifactStorage[A artifact.Artifact, D delta.ArtifactDelta] interface {
	LoadArtifact(identifier string) (A, error)
	StoreArtifact(artifact A, identifier string) error
	StoreDelta(d delta.ArtifactDelta, identifier string) error
	LoadDelta(identifier string) (D, error)
}

// FilesystemStorage implements the ArtifactStorage interface.
// It loads and stores artifacts from the file system relative to the specified basePath.
type FilesystemStorage struct {
	BasePath string
}

func (s *FilesystemStorage) LoadArtifact(identifier string) (artifact.RawBytesArtifact, error) {
	data, err := s.loadFile(identifier)
	if err != nil {
		return artifact.RawBytesArtifact{}, fmt.Errorf("could not read artifact file from `%s`: %w", identifier, err)
	}
	return artifact.RawBytesArtifact{Data: data}, nil
}

func (s *FilesystemStorage) StoreArtifact(artifact artifact.Artifact, identifier string) error {
	err := s.storeFile(artifact.GetReader(), identifier)
	if err != nil {
		return fmt.Errorf("could not store artifact file at `%s`: %w", identifier, err)
	}
	return nil
}

func (s *FilesystemStorage) StoreDelta(d delta.ArtifactDelta, identifier string) error {
	err := s.storeFile(d.GetReader(), identifier)
	if err != nil {
		return fmt.Errorf("could not store delta file at `%s`: %w", identifier, err)
	}
	return nil
}

func (s *FilesystemStorage) LoadDelta(identifier string) (delta.ArtifactDelta, error) {
	data, err := s.loadFile(identifier)
	if err != nil {
		return delta.RawDiff{}, fmt.Errorf("could not read delta file from `%s`: %w", identifier, err)
	}
	return delta.RawDiff{Data: data}, nil

}

func (s *FilesystemStorage) loadFile(fPath string) ([]byte, error) {
	fPath = filepath.Join(s.BasePath, fPath)
	data, err := os.ReadFile(fPath)
	if err != nil {
		return data, err
	}
	return data, nil
}

func (s *FilesystemStorage) storeFile(r io.Reader, fPath string) error {
	fPath = filepath.Join(s.BasePath, fPath)

	f, err := os.Create(fPath)
	if err != nil {
		return fmt.Errorf("could not create file at `%s`: %w", fPath, err)
	}
	defer f.Close()
	buf := make([]byte, 8096)
	for {
		nRead, errRead := r.Read(buf)
		nWrite, errWrite := f.Write(buf[:nRead])
		if errRead == io.EOF {
			break
		}
		if errWrite != nil {
			return fmt.Errorf("could not write to file at `%s`: %w", fPath, errRead)
		}
		// TODO: does this case matter?
		if nRead > nWrite {
			return fmt.Errorf("wrote fewer bytes than expected")
		}
	}
	return nil
}
