package bsdiff

import (
	"crypto/sha256"
	"fmt"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"io"
	"os"

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
}

func (a *patcher) PatchFilesystem(artifactPath string, patch io.Reader, expected *digest.Digest) error {
	fstat, err := os.Stat(artifactPath)
	if err != nil {
		return err
	}
	fpOld, err := os.Open(artifactPath)
	if err != nil {
		return err
	}
	defer funcutils.PanicOrLogOnErr(fpOld.Close, false, "failed to close file")

	fpTemp, err := os.CreateTemp(os.TempDir(), "bsdiff-temp-*")
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
	err = fileutils.ReplaceFile(fpTemp.Name(), artifactPath)
	if err != nil {
		return err
	}
	// maintain permissions
	err = os.Chmod(artifactPath, fstat.Mode())
	if err != nil {
		return err
	}
	return nil
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
