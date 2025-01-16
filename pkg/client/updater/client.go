package updater

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"hash"
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

// DorasState is an interface that can be used to get/set the internal state of a Doras update client.
type DorasState interface {
	// GetCurrentImage returns the current OCI image that is in use/rolled out.
	GetCurrentImage() (string, error)
	// SetCurrentImage to the new OCI image that is from now in use/rolled out.
	SetCurrentImage(image string) error
}

// Client is used to run delta updates in Doras.
type Client struct {
	opts       clientOpts
	edgeClient *edgeapi.Client
	state      DorasState
	reg        RegistryDelegate
}

// Pull an image from the registry.
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
	mf, err := ociutils.ParseManifestJSON(mfReader)
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
	return *mf, fp, nil
}

// ensureSubDir makes sure the directory at p exists, relative to the base directory.
func (r *registryImpl) ensureSubDir(p string) (string, error) {
	dir := path.Join(r.workingDir, p)
	err := os.MkdirAll(dir, 0750)
	if err != nil {
		return "", err
	}
	return dir, nil
}

func (r *registryImpl) ingest(expected v1.Descriptor, content io.ReadCloser) (string, error) {
	// Make sure directories exist and construct paths.
	fPathDownload, fPathCompleted, err := r.setupIngestDirAndReturnPaths(expected)
	if err != nil {
		return "", err
	}

	fp, n, err := openFileAndGetSize(fPathDownload)
	if err != nil {
		return "", err
	}
	var h = sha256.New()

	// Make sure pre-existing content is written to the hasher.
	h, err = hashOldContents(content, n, h, fp)
	if err != nil {
		return "", err
	}

	// Use a writer that makes sure the changes are flushed to the disk.
	// This is done to increase robustness against power loss during ingesting which might cause inconsistencies.
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

func hashOldContents(content io.ReadCloser, bytesExpected int64, h hash.Hash, fp *os.File) (hash.Hash, error) {
	if bytesExpected == 0 {
		return h, nil
	}
	// Check if the file exists already.
	// If it exists try to seek on the content and write the existing bytes to the hasher.
	if seeker, ok := content.(io.Seeker); ok {
		_, err := seeker.Seek(bytesExpected, io.SeekStart)
		if err != nil {
			return nil, err
		}
		nHashed, err := io.Copy(h, fp)
		if err != nil {
			return nil, err
		}
		if nHashed != bytesExpected {
			return nil, fmt.Errorf("failed to read all bytes from existing file, got: %d, wanted: %d", nHashed, bytesExpected)
		}
		return h, nil
	}
	return h, nil
}

func openFileAndGetSize(fPath string) (*os.File, int64, error) {
	fp, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, 0, err
	}
	stat, err := fp.Stat()
	if err != nil {
		return nil, 0, err
	}
	n := stat.Size()
	return fp, n, nil
}

func (r *registryImpl) setupIngestDirAndReturnPaths(expected v1.Descriptor) (fPathDownload string, fPathCompleted string, err error) {
	downloadDir, err := r.ensureSubDir("download")
	if err != nil {
		return "", "", err
	}
	completedDir, err := r.ensureSubDir("completed")
	if err != nil {
		return "", "", err
	}
	fPathDownload = path.Join(downloadDir, expected.Digest.Encoded())
	fPathCompleted = path.Join(completedDir, expected.Digest.Encoded())
	return fPathDownload, fPathCompleted, nil
}

// Patcher handles patching files or directories for provided deltas.
type Patcher interface {
	// PatchFile located at the path  with the provided delta.
	PatchFile(fPath string, delta io.Reader) error
	// PatchDirectory located at the path with the provided delta.
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

func (p *patcherImpl) PatchDirectory(_ string, _ io.Reader) {
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
		choice.Patcher = bsdiff.NewPatcher()
	case "tardiff":
		choice.Patcher = tardiff.NewPatcher()
	default:
		return algorithmchoice.PatcherChoice{}, fmt.Errorf("unsupported patcher: %s", split[0])
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
	res, exists, err := c.edgeClient.ReadDeltaAsync(oldImage, target, c.opts.AcceptedAlgorithms)
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
