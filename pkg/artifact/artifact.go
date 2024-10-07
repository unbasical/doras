package artifact

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// TODO: put load/store into a different interface, e.g. `ArtifactLoader`
type Artifact interface {
	LoadArtifact(path string) (Artifact, error)
	StoreArtifact(path string) error
	GetReader() io.Reader
}

type RawBytesArtifact struct {
	Data []byte
}

func (a RawBytesArtifact) LoadArtifact(path string) (Artifact, error) {
	//TODO remove once interface is refactored
	panic("implement me")
}

func (a RawBytesArtifact) GetReader() io.Reader {
	return bytes.NewReader(a.Data)
}

func LoadArtifact(path string) (RawBytesArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RawBytesArtifact{}, fmt.Errorf("could not read artifact file from `%s`: %w", path, err)
	}
	return RawBytesArtifact{data}, nil
}

func (a RawBytesArtifact) StoreArtifact(path string) error {
	err := os.WriteFile(path, a.Data, 0644)
	if err != nil {
		return fmt.Errorf("could not write artifact file to `%s`: %w", path, err)
	}
	return nil
}
