package delta

import (
	"bytes"
	"io"
)

type ArtifactDelta interface {
	GetReader() io.Reader
	GetBytes() ([]byte, error)
}

type RawDiff struct {
	Data []byte
}

func (r RawDiff) GetBytes() ([]byte, error) {
	return r.Data, nil
}

func (r RawDiff) GetReader() io.Reader {
	return bytes.NewReader(r.Data)
}

type DiffFile struct {
	Data []DiffSlice
}

func (f DiffFile) Equals(other DiffFile) bool {
	if len(f.Data) != len(other.Data) {
		return false
	}
	for i, s := range f.Data {
		if !s.Equal(&other.Data[i]) {
			return false
		}
	}
	return true
}

type DiffSlice struct {
	StartAt int
	Data    []byte
}

func (s DiffSlice) Equal(d *DiffSlice) bool {
	return s.StartAt == d.StartAt && bytes.Equal(s.Data, d.Data)
}
