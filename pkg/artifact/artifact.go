package artifact

import (
	"bytes"
	"io"
)

type Artifact interface {
	GetReader() io.Reader
}

type RawBytesArtifact struct {
	Data []byte
}

func (a RawBytesArtifact) GetReader() io.Reader {
	return bytes.NewReader(a.Data)
}

func (a RawBytesArtifact) Equals(got RawBytesArtifact) bool {
	return bytes.Equal(a.Data, got.Data)
}
