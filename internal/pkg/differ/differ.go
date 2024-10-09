package differ

import (
	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	"github.com/gabstv/go-bsdiff/pkg/bspatch"
	"github.com/unbasical/doras-server/internal/pkg/artifact"
)

type Differ interface {
	CreateDiff(from artifact.Artifact, to artifact.Artifact) []byte
	ApplyDiff(from artifact.Artifact, diff []byte) artifact.Artifact
}

type Bsdiff struct {
}

func (b Bsdiff) CreateDiff(from artifact.Artifact, to artifact.Artifact) []byte {
	//TODO find better solution for readers and make sure entire reader is read
	fromBuf := make([]byte, 0xffff)
	nFrom, _ := from.GetReader().Read(fromBuf)
	toBuf := make([]byte, 0xffff)
	nTo, _ := to.GetReader().Read(toBuf)
	patch, err := bsdiff.Bytes(fromBuf[:nFrom], toBuf[:nTo])
	if err != nil {
		panic(err)
	}
	return patch
}

func (b Bsdiff) ApplyDiff(from artifact.Artifact, diff []byte) artifact.Artifact {
	//TODO implement me
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
