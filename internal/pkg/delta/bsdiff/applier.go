package bsdiff

import (
	"io"

	"github.com/gabstv/go-bsdiff/pkg/bspatch"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
)

func Bspatch(old io.Reader, patch io.Reader) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		err := bspatch.Reader(old, pw, patch)
		if err != nil {
			errInner := pw.CloseWithError(err)
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(errInner), false, "failed to close pipe writer after error")
		}
		funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")
	}()
	return pr, nil
}

type Applier struct {
}

func (a *Applier) Patch(old io.Reader, new io.Reader) (io.Reader, error) {
	return Bspatch(old, new)
}
func (a *Applier) Name() string {
	return "bsdiff"
}
