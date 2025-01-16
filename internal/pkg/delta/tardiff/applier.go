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
	dataSource, err := tarfsdatasource.NewDataSource(old, func(reader io.Reader) (io.Reader, error) {
		gzr, err := gzip.NewReader(reader)
		if err != nil {
			err := pw.CloseWithError(err)
			if err != nil {
				log.WithError(err).Error("error closing gzip stream")
			}
			return nil, err
		}
		return gzr, nil
	})
	if err != nil {
		_ = pr.Close()
		return nil, err
	}
	go func() {
		err := tarpatch.Apply(patch, dataSource, pw)
		if err != nil {
			errInner := pw.CloseWithError(err)
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(errInner), false, "failed to close pipe writer after error")
		}
		funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")
		if ds, ok := dataSource.(*tarfsdatasource.DataSource); ok {
			funcutils.PanicOrLogOnErr(ds.CloseDataSource, false, "failed to close data source after patching")
		}
	}()
	return pr, nil
}
func (a *applier) Name() string {
	return "tardiff"
}
