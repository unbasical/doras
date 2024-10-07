package delta

import (
	"bytes"
	"github.com/unbasical/doras-server/pkg/artifact"
	"io"
)

type DiffFile struct {
	data []DiffSlice
}

type DiffSlice struct {
	startAt int
	data    []byte
}

type Differ interface {
	CreateDiff(from artifact.Artifact, to artifact.Artifact) DiffFile
	ApplyDiff(from artifact.Artifact, diff DiffFile) artifact.Artifact
}

type NaiveDiffer struct {
}

func (d *NaiveDiffer) CreateDiff(from artifact.Artifact, to artifact.Artifact) DiffFile {
	fromReader := from.GetReader()
	toReader := to.GetReader()
	result := make([]DiffSlice, 0)
	bFrom := make([]byte, 32)
	bTo := make([]byte, 32)
	idx := 0
	for {
		nTo, errTo := toReader.Read(bTo)
		nFrom, errFrom := fromReader.Read(bFrom)
		if bytes.Equal(bFrom[:nFrom], bTo[:nTo]) && errTo == nil && errFrom == nil {
			idx += max(nTo, nFrom)
			continue
		}
		if nTo > 0 {
			result = append(result, DiffSlice{startAt: idx, data: bFrom[:nFrom]})
		}
		if errTo == io.EOF {
			break
		}
		idx += max(nTo, nFrom)
	}
	return DiffFile{data: result}
}

func (d *NaiveDiffer) ApplyDiff(from artifact.RawBytesArtifact, diff DiffFile) artifact.RawBytesArtifact {
	data := make([]byte, 0)
	cursor := 0
	for _, d := range diff.data {
		data = append(data, from.Data[cursor:d.startAt]...)
		data = append(data, d.data...)
		cursor += d.startAt - cursor
		cursor += len(d.data)
	}
	return artifact.RawBytesArtifact{
		Data: data,
	}
}
