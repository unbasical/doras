package bsdiff

import (
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"io"

	bsdiff2 "github.com/gabstv/go-bsdiff/pkg/bsdiff"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
)

type creator struct {
}

// NewCreator returns a bsdiff delta.Differ.
func NewCreator() delta.Differ {
	return &creator{}
}

func (c *creator) Diff(oldfile io.Reader, newfile io.Reader) (io.ReadCloser, error) {
	// Use a pipe to turn the writer into a reader.
	pr, pw := io.Pipe()
	go func() {
		err := bsdiff2.Reader(oldfile, newfile, pw)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		funcutils.PanicOrLogOnErr(pw.Close, false, "failed to close pipe writer")
	}()
	return pr, nil
}

func (c *creator) Name() string {
	return "bsdiff"
}
