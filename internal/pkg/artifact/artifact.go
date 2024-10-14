package artifact

import (
	"bytes"
	"io"
)

type Artifact interface {
	GetReader() io.Reader
	GetBytes() []byte
	GetContentLength() int
}

type RawBytesArtifact struct {
	Data []byte
}

func (a *RawBytesArtifact) GetReader() io.Reader {
	return bytes.NewReader(a.Data)
}

func (a *RawBytesArtifact) GetBytes() []byte {
	return a.Data
}

func (a *RawBytesArtifact) GetContentLength() int {
	return len(a.Data)
}

func (a *RawBytesArtifact) Equals(got Artifact) bool {
	return bytes.Equal(a.Data, got.GetBytes())
}
