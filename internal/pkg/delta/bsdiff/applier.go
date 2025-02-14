package bsdiff

import (
	"crypto/sha256"
	"fmt"
	"github.com/samber/lo"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"io"
	"os"
	"path"

	"github.com/opencontainers/go-digest"
	"github.com/unbasical/doras/pkg/algorithm/delta"

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
	tmpDir string
}

func (a *patcher) PatchFilesystem(artifactDir string, patch io.Reader, expected *digest.Digest) error {
	fstat, err := os.Stat(artifactDir)
	if err != nil {
		return err
	}
	if !fstat.IsDir() {
		return fmt.Errorf("%s is not a directory", artifactDir)
	}
	files, err := os.ReadDir(artifactDir)
	if err != nil {
		return err
	}
	files = lo.Filter(files, func(item os.DirEntry, _ int) bool {
		return !item.IsDir()
	})
	if len(files) != 1 {
		return fmt.Errorf("expected a single file, got %d", len(files))
	}
	fpOld, err := os.Open(path.Join(artifactDir, files[0].Name()))
	if err != nil {
		return err
	}
	defer funcutils.PanicOrLogOnErr(fpOld.Close, false, "failed to close file")

	fpTemp, err := os.CreateTemp(a.tmpDir, "bsdiff-temp-*")
	if err != nil {
		return err
	}
	defer func() {
		// this removes the temp file if there is an error elsewhere
		// if there is no error elsewhere this will cause an error on removal (as intended)
		_ = os.Remove(fpTemp.Name())
	}()
	patchedFileReader, err := a.Patch(fpOld, patch)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	var w io.Writer = fpTemp
	if expected != nil {
		w = io.MultiWriter(fpTemp, hasher)
	}
	_, err = io.Copy(w, patchedFileReader)
	if err != nil {
		return err
	}
	if expected != nil && digest.NewDigest("sha256", hasher) != *expected {
		return fmt.Errorf("expected sha256 digest %v, got %v", expected, digest.NewDigest("sha256", hasher))
	}
	// Make sure file is written to the disk before we swap files.
	err = fpTemp.Sync()
	if err != nil {
		return err
	}
	err = fileutils.ReplaceFile(fpTemp.Name(), fpOld.Name())
	if err != nil {
		return err
	}
	// maintain permissions
	err = os.Chmod(artifactDir, fstat.Mode())
	if err != nil {
		return err
	}
	return nil
}

// NewPatcher return a bsdiff delta.Patcher.
func NewPatcher() delta.Patcher {
	return &patcher{
		tmpDir: os.TempDir(),
	}
}

// NewPatcherWithTempDir return a bsdiff delta.Patcher.
func NewPatcherWithTempDir(tmpDir string) delta.Patcher {
	return &patcher{
		tmpDir: tmpDir,
	}
}

func (a *patcher) Patch(oldfile io.Reader, newfile io.Reader) (io.Reader, error) {
	return bspatch(oldfile, newfile)
}
func (a *patcher) Name() string {
	return "bsdiff"
}
