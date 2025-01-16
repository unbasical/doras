package tardiff

import (
	"compress/gzip"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/pkg/algorithm/delta"
	"io"

	tarpatch "github.com/containers/tar-diff/pkg/tar-patch"

	"github.com/unbasical/doras-server/internal/pkg/delta/tarfsdatasource"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
)

type applier struct {
}

// NewPatcher return a tardiff delta.Patcher.
func NewPatcher() delta.Patcher {
	return &applier{}
}

func (a *applier) Patch(old io.Reader, patch io.Reader) (io.Reader, error) {
	pr, pw := io.Pipe()
	// Create a file system backed tar data source.
	// This could be possibly improved using zstd seekable compression.
	dataSource := tarfsdatasource.New(old, func(reader io.Reader) io.Reader {
		gzr, err := gzip.NewReader(reader)
		if err != nil {
			err := pw.CloseWithError(err)
			if err != nil {
				log.WithError(err).Error("error closing gzip stream")
			}
			return nil
		}
		return gzr
	})
	// This implies an error in the previous block.
	// Not the nicest way to do it, but the first call to read the PR will error.
	if dataSource == nil {
		return pr, nil
	}
	go func() {
		err := tarpatch.Apply(patch, dataSource, pw)
		if err != nil {
			errInner := pw.CloseWithError(err)
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(errInner), false, "failed to close pipe writer after error")
		}
		funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")
	}()
	return pr, nil
}
func (a *applier) Name() string {
	return "tardiff"
}
