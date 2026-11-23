package tardiff

import (
	"compress/gzip"
	"io"
	"os"

	"github.com/unbasical/doras/internal/pkg/utils/fileutils"

	tarpatch "github.com/containers/tar-diff/pkg/tar-patch"
	"github.com/opencontainers/go-digest"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras/internal/pkg/utils/tarutils"
	"github.com/unbasical/doras/internal/pkg/utils/writerutils"
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"github.com/unbasical/doras/pkg/constants"

	"github.com/unbasical/doras/internal/pkg/delta/tarfsdatasource"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
)

type applier struct {
	tmpDir               string
	keepOldDir           bool
	outputDirPermissions os.FileMode
}

func (a *applier) PatchFilesystem(artifactDir string, patch io.Reader, expected *digest.Digest) error {
	datasource := tarpatch.NewFilesystemDataSource(artifactDir)
	tempfile, err := os.CreateTemp(a.tmpDir, constants.TempPrefixTarpatch+"*")
	if err != nil {
		return err
	}
	defer func() {
		// this removes the temp file if there is an error elsewhere
		// if there is no error elsewhere this will cause an error on removal (as intended)
		_ = os.Remove(tempfile.Name())
	}()
	writer := writerutils.NewSafeFileWriter(tempfile)
	defer funcutils.PanicOrLogOnErr(writer.Close, false, "failed to close safe file writer")
	err = tarpatch.Apply(patch, datasource, writer)
	if err != nil {
		return err
	}
	extractDir, err := os.MkdirTemp(a.tmpDir, constants.TempPrefixTarExtract+"*")
	if err != nil {
		return err
	}
	err = os.Chmod(extractDir, a.outputDirPermissions)
	if err != nil {
		return err
	}
	err = os.Chmod(artifactDir, a.outputDirPermissions)
	if err != nil {
		return err
	}
	defer func() {
		// this removes the temp dir if there is an error elsewhere
		// if there is no error elsewhere this will cause an error on removal (as intended)
		_ = os.RemoveAll(extractDir)
	}()
	err = tarutils.ExtractCompressedTar(extractDir, "", tempfile.Name(), expected, compressionutils.NewNopDecompressor())
	if err != nil {
		return err
	}
	if a.keepOldDir {
		return fileutils.ReplaceDirectoryInPlace(extractDir, artifactDir)
	}
	err = fileutils.ReplaceDirectory(extractDir, artifactDir)
	if err != nil {
		return err
	}
	return nil
}

// NewPatcher return a tardiff delta.Patcher.
func NewPatcher() delta.Patcher {
	return &applier{
		tmpDir:               os.TempDir(),
		outputDirPermissions: 0755,
	}
}

// NewPatcherWithTempDir return a tardiff delta.Patcher.
func NewPatcherWithTempDir(tmpDir string, keepOldDir bool, outputDirPermissions os.FileMode) delta.Patcher {
	return &applier{
		tmpDir:               tmpDir,
		keepOldDir:           keepOldDir,
		outputDirPermissions: outputDirPermissions,
	}
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
