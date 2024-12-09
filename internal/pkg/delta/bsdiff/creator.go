package bsdiff

import (
	"io"

	"github.com/unbasical/doras-server/pkg/delta"

	bsdiff2 "github.com/gabstv/go-bsdiff/pkg/bsdiff"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
)

type creator struct {
}

func NewCreator() delta.Differ {
	return &creator{}
}

func (c *creator) Diff(old io.Reader, new io.Reader) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		err := bsdiff2.Reader(old, new, pw)
		funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(err), true, "failed bsdiff creation")
		funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")

	}()
	return pr, nil
}
