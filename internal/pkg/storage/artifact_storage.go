package storage

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/artifact"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/internal/pkg/utils"
	"io"
	"os"
	"path/filepath"
)

// ArtifactStorage is an interface that abstracts loading and storing artifacts.
type ArtifactStorage interface {
	// LoadArtifact
	//The implementation has to handle the sanitization of the identifier.
	LoadArtifact(identifier string) (artifact.Artifact, error)
	// StoreArtifact
	//The implementation has to handle the sanitization of the identifier.
	StoreArtifact(artifact artifact.Artifact, identifier string) error
	// StoreDelta stores the delta with the provided identifier.
	//The implementation has to handle the sanitization of the identifier.
	StoreDelta(d delta.ArtifactDelta, identifier string) error
	// LoadDelta
	//The implementation has to handle the sanitization of the identifier.
	LoadDelta(identifier string) (delta.ArtifactDelta, error)
}

// FilesystemStorage implements the ArtifactStorage interface.
// It loads and stores artifacts from the file system relative to the specified basePath.
type FilesystemStorage struct {
	BasePath string
}

func (s *FilesystemStorage) LoadArtifact(identifier string) (artifact.Artifact, error) {
	data, err := s.loadFile(identifier)
	if err != nil {
		return &artifact.RawBytesArtifact{}, fmt.Errorf("could not read artifact file from `%s`: %w", identifier, err)
	}
	return &artifact.RawBytesArtifact{Data: data}, nil
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
	fPathJoined := filepath.Join(s.BasePath, fPath)
	fPathClean, err := utils.VerifyPath(fPathJoined, s.BasePath, true)
	if err != nil {
		log.Errorf("sanitization failure: %s", err)
		return nil, fmt.Errorf("could not verify provided path `%s` reason: %w", fPath, err)
	}
	log.Debugf("loading file `%s`", fPathClean)
	data, err := os.ReadFile(fPathClean)
	if err != nil {
		log.Errorf("could not read file `%s`: %s", fPath, err)
		return data, err
	}
	return data, nil
}

func (s *FilesystemStorage) storeFile(r io.Reader, fPath string) error {
	fPathJoined := filepath.Join(s.BasePath, fPath)
	log.Debugf("joined path is `%s`", fPathJoined)
	fPathClean, err := utils.VerifyPath(fPathJoined, s.BasePath, false)
	if err != nil {
		log.Errorf("sanitization failure: %s", err)
		return fmt.Errorf("could not verify provided path `%s` reason: %w", fPath, err)
	}
	log.Debugf("attempting to store file at `%s`", fPathClean)
	f, err := os.Create(fPathClean)
	if err != nil {
		log.Errorf("could not create file `%s`: %s", fPath, err)
		return fmt.Errorf("could not create file at `%s`: %w", fPathClean, err)
	}
	defer f.Close()
	buf := make([]byte, 8096)
	for {
		nRead, errRead := r.Read(buf)
		nWrite, errWrite := f.Write(buf[:nRead])
		if errRead == io.EOF {
			log.Debug("reached EOF")
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
	log.Debugf("successfully stored file at `%s`", fPath)
	return nil
}
