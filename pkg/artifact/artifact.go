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
