package tardiff

import (
	"compress/gzip"
	tar_patch "github.com/containers/tar-diff/pkg/tar-patch"
	"io"

	"github.com/unbasical/doras-server/internal/pkg/delta/tarfsdatasource"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
)

type Applier struct {
}

func (a *Applier) Apply(old io.Reader, patch io.Reader) (io.Reader, error) {
	pr, pw := io.Pipe()
	dataSource := tarfsdatasource.New(old, func(reader io.Reader) io.Reader {
		gzr, err := gzip.NewReader(reader)
		if err != nil {
			panic(err)
		}
		return gzr
	})
	go func() {
		err := tar_patch.Apply(patch, dataSource, pw)
		if err != nil {
			errInner := pw.CloseWithError(err)
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(errInner), false, "failed to close pipe writer after error")
		}
		funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")
	}()
	return pr, nil
}
