package differ

import (
	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	"github.com/gabstv/go-bsdiff/pkg/bspatch"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/artifact"
)

type Differ interface {
	CreateDiff(from artifact.Artifact, to artifact.Artifact) []byte
	ApplyDiff(from artifact.Artifact, diff []byte) artifact.Artifact
}

type Bsdiff struct {
}

func (b Bsdiff) CreateDiff(from artifact.Artifact, to artifact.Artifact) []byte {
	log.Debug("creating bsdiff")
	patch, err := bsdiff.Bytes(from.GetBytes(), to.GetBytes())
	if err != nil {
		panic(err)
	}
	return patch
}

func (b Bsdiff) ApplyDiff(from artifact.Artifact, diff []byte) artifact.Artifact {
	log.Debug("applying bsdiff")
	fromBuf := make([]byte, 0xffff)
	nFrom, _ := from.GetReader().Read(fromBuf)
	to, err := bspatch.Bytes(fromBuf[:nFrom], diff)
	if err != nil {
		panic(err)
	}
	return artifact.RawBytesArtifact{
		Data: to,
	}
}
