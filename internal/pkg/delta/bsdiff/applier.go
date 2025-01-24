package bsdiff

import (
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"io"

	bspatchdep "github.com/gabstv/go-bsdiff/pkg/bspatch"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
)

func bspatch(old io.Reader, patch io.Reader) (io.ReadCloser, error) {
	// Use pipes to turn the writer into a reader.
	pr, pw := io.Pipe()
	go func() {
		err := bspatchdep.Reader(old, pw, patch)
		if err != nil {
			errInner := pw.CloseWithError(err)
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(errInner), false, "failed to close pipe writer after error")
		}
		funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")
	}()
	return pr, nil
}

type patcher struct {
}

// NewPatcher return a bsdiff delta.Patcher.
func NewPatcher() delta.Patcher {
	return &patcher{}
}

func (a *patcher) Patch(oldfile io.Reader, newfile io.Reader) (io.Reader, error) {
	return bspatch(oldfile, newfile)
}
func (a *patcher) Name() string {
	return "bsdiff"
}
