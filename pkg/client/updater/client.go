package updater

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/unbasical/doras-server/internal/pkg/compression/gzip"
	"github.com/unbasical/doras-server/internal/pkg/compression/zstd"
	"github.com/unbasical/doras-server/internal/pkg/delta/bsdiff"
	"github.com/unbasical/doras-server/internal/pkg/delta/tardiff"
	"github.com/unbasical/doras-server/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras-server/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras-server/internal/pkg/utils/writerutils"
	"oras.land/oras-go/v2/registry/remote"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/algorithmchoice"
	"github.com/unbasical/doras-server/pkg/client/edgeapi"
)

type dorasStateData struct {
	CurrentImage    string `json:"current-image"`
	CurrentManifest string `json:"current-manifest"`
}
type dorasState struct {
	fPath string
}

func (d *dorasState) GetCurrentImage() (string, error) {
	var data dorasStateData
	available, err := fileutils.SafeReadJSON(d.fPath, &data, os.ModeExclusive)
	if err != nil || !available {
		return "", errors.New("not initialized yet")
	}
	return data.CurrentImage, nil
}

func (d *dorasState) SetCurrentImage(image string) error {
	data := dorasStateData{CurrentImage: image}
	err := fileutils.SafeWriteJson(d.fPath, &data)
	if err != nil {
		return err
	}
	return nil
}

type DorasState interface {
	GetCurrentImage() (string, error)
	SetCurrentImage(image string) error
}

type Client struct {
	opts               clientOpts
	edgeClient         *edgeapi.Client
	acceptedAlgorithms []string
	state              DorasState
	reg                RegistryDelegate
}

func (c *Client) Pull(image string) error {
	for {
		exists, err := c.PullAsync(image)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
	}
}

// RegistryDelegate is used to load artifacts from a registry.
// This should handle partial downloads if possible.
type RegistryDelegate interface {
	ResolveAndLoad(image string) (v1.Manifest, io.ReadCloser, error)
}

type registryImpl struct {
	workingDir string
}

func (r *registryImpl) ResolveAndLoad(image string) (v1.Manifest, io.ReadCloser, error) {
	name, tag, isDigest, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return v1.Manifest{}, nil, err
	}
	if !isDigest {
		return v1.Manifest{}, nil, errors.New("expected image with digest")
	}
	reg, err := remote.NewRepository(name)
	if err != nil {
		return v1.Manifest{}, nil, err
	}
	_, mfReader, err := reg.FetchReference(context.Background(), tag)
	if err != nil {
		return v1.Manifest{}, nil, err
	}
	defer funcutils.PanicOrLogOnErr(mfReader.Close, false, "failed to close reader")
	mf, err := ociutils.ParseManifest(mfReader)
	if err != nil {
		return v1.Manifest{}, nil, err
	}
	rc, err := reg.Fetch(context.Background(), mf.Layers[0])
	if err != nil {
		return v1.Manifest{}, nil, err
	}
	fPath, err := r.ingest(mf.Layers[0], rc)
	if err != nil {
		return v1.Manifest{}, nil, err
	}
	fp, err := os.Open(fPath)
	if err != nil {
		return v1.Manifest{}, nil, err
	}
	return mf, fp, nil
}

func (r *registryImpl) ensureSubDir(p string) (string, error) {
	dir := path.Join(r.workingDir, p)
	err := os.MkdirAll(dir, 0750)
	if err != nil {
		return "", err
	}
	return dir, nil
}

func (r *registryImpl) ingest(expected v1.Descriptor, content io.ReadCloser) (string, error) {
	downloadDir, err := r.ensureSubDir("download")
	if err != nil {
		return "", err
	}
	completedDir, err := r.ensureSubDir("completed")
	if err != nil {
		return "", err
	}
	fPathDownload := path.Join(downloadDir, expected.Digest.Encoded())
	fPathCompleted := path.Join(completedDir, expected.Digest.Encoded())

	fp, err := os.OpenFile(fPathDownload, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	stat, err := fp.Stat()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n := stat.Size()
	// Check if the file exists already.
	// If it exists try to seek on the content and write the existing bytes to the hasher.
	if n > 0 {
		if seeker, ok := content.(io.Seeker); ok {
			_, err = seeker.Seek(n, io.SeekStart)
			if err != nil {
				return "", err
			}
			nHashed, err := io.Copy(h, fp)
			if err != nil {
				return "", err
			}
			if nHashed != n {
				return "", fmt.Errorf("failed to read all bytes from existing file, got: %d, wanted: %d", nHashed, n)
			}
		}
	}

	w := writerutils.NewSafeFileWriter(fp)
	tr := io.TeeReader(content, h)
	nNew, err := io.Copy(w, tr)
	if err != nil {
		return "", err
	}
	if bytesRead := nNew + n; bytesRead != expected.Size {
		return "", errors.New("unexpected EOF")
	}
	if digest.NewDigest("sha256", h) != expected.Digest {
		return "", errors.New("unexpected digest")
	}
	// Do not defer close to make sure file is written to the disk.
	err = w.Close()
	if err != nil {
		return "", err
	}
	// Move to new completed dir
	// This happens after the file was downloaded and written to the disk entirely.
	err = os.Rename(fPathDownload, fPathCompleted)
	if err != nil {
		return "", err
	}
	return fPathCompleted, nil
}

type Patcher interface {
	PatchFile(fPath string, delta io.Reader) error
	PatchDirectory(fPath string, delta io.Reader)
}

type patcherImpl struct {
	directory string
	algos     algorithmchoice.PatcherChoice
	mf        *v1.Manifest
}

func (p *patcherImpl) PatchFile(fPath string, delta io.Reader) error {
	deltaDecompressed, err := p.algos.Decompress(delta)
	if err != nil {
		return err
	}
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, fPath+"_*")
	if err != nil {
		return err
	}
	fpath := filepath.Join(p.directory, fPath)
	fpOld, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer funcutils.PanicOrLogOnErr(fpOld.Close, false, "failed to close reader")
	patchedFile, err := p.algos.Patch(fpOld, deltaDecompressed)
	if err != nil {
		return err
	}
	h := sha256.New()
	patchedFile = io.TeeReader(patchedFile, h)
	w := writerutils.NewSafeFileWriter(tempFile)
	defer funcutils.PanicOrLogOnErr(w.Close, false, "failed to close writer")
	n, err := io.Copy(w, patchedFile)
	if err != nil {
		return err
	}
	if n != p.mf.Layers[0].Size {
		return errors.New("unexpected EOF")
	}
	gotDigest := digest.NewDigest("sha256", h)
	if gotDigest != p.mf.Layers[0].Digest {
		return errors.New("unexpected digest")
	}
	err = os.Rename(tempFile.Name(), fPath)
	if err != nil {
		return err
	}
	return nil
}

func (p *patcherImpl) PatchDirectory(fPath string, delta io.Reader) {
	//TODO implement me
	panic("implement me")
}

func newPatcher(workingDir string, algos algorithmchoice.PatcherChoice, mf *v1.Manifest) Patcher {
	return &patcherImpl{
		directory: workingDir,
		algos:     algos,
		mf:        mf,
	}
}

func getPatcherChoice(mf *v1.Manifest) (algorithmchoice.PatcherChoice, error) {
	choice := algorithmchoice.PatcherChoice{}
	if len(mf.Layers) != 1 {
		return choice, fmt.Errorf("manifest must contain exactly one layer")
	}
	deltaAnnotations := mf.Layers[0]
	trimmed := strings.TrimPrefix(deltaAnnotations.MediaType, "application/")
	split := strings.Split(trimmed, "+")
	if len(split) == 2 {
		switch split[1] {
		case "gzip":
			choice.Decompressor = gzip.NewDecompressor()
		case "zstd":
			choice.Decompressor = zstd.NewDecompressor()
		default:
			panic("not supported")
		}
	}
	switch split[0] {
	case "bsdiff":
		choice.Patcher = &bsdiff.Applier{}
	case "tardiff":
		choice.Patcher = &tardiff.Applier{}
	default:
		panic("not supported")
	}
	return choice, nil
}

// PullAsync Pull delta, but do not block if the delta has not been created yet.
// The result of the pull is according to the client configuration.
func (c *Client) PullAsync(target string) (exists bool, err error) {
	// TODO: compare to current image
	oldImage, err := c.state.GetCurrentImage()
	if err != nil {
		panic("handling uninitialized state is not yet implemented")
	}
	res, exists, err := c.edgeClient.ReadDeltaAsync(oldImage, target, c.acceptedAlgorithms)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	mf, delta, err := c.reg.ResolveAndLoad(res.TargetImage)
	if err != nil {
		return false, err
	}
	fPath, _, err := ociutils.ExtractPathFromManifest(&mf)
	if err != nil {
		return false, err
	}
	patcherChoice, err := getPatcherChoice(&mf)
	if err != nil {
		return false, err
	}
	patcher := newPatcher(c.opts.OutputDirectory, patcherChoice, &mf)
	err = patcher.PatchFile(fPath, delta)
	if err != nil {
		return false, err
	}
	err = c.state.SetCurrentImage(res.TargetImage)
	if err != nil {
		return false, err
	}
	return true, nil
}
