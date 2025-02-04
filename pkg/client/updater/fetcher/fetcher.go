package fetcher

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras/internal/pkg/utils/writerutils"
	"github.com/unbasical/doras/pkg/constants"
	"hash"
	"io"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"os"
	"path"
)

type ArtifactFetcher interface {
	FetchDelta(currentVersion, image, outDir string) (algo, fPath string, err error)
	FetchArtifact(image, outDir string) (fPath string, err error)
}

type fetcher struct {
	auth.CredentialFunc
}

func (f *fetcher) FetchDelta(currentVersion, image, outDir string) (algo, fPath string, err error) {
	//TODO implement me
	name, _, isDigest, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return "", "", err
	}
	if !isDigest {
		return "", "", fmt.Errorf("image %s is identifeid by a digest", image)
	}
	_, err = remote.NewRepository(name)
	if err != nil {
		return "", "", err
	}
	// use ingest here
	panic("todo")
}

func (f *fetcher) FetchArtifact(image, outDir string) (fPath string, err error) {
	//TODO implement me
	panic("implement me")
}

// RegistryDelegate is used to load artifacts from a registry.
// This should handle partial downloads if possible.
type RegistryDelegate interface {
	ResolveAndLoad(image string) (ociutils.Manifest, io.ReadCloser, error)
	ResolveAndLoadToPath(image, outputDir string) (ociutils.Manifest, []LoadResult, error)
}

type registryImpl struct {
	workingDir string
}

var errSeekNotSupported = errors.New("reader does not support seeking")

type LoadResult struct {
	D    v1.Descriptor
	Path string
}

func (r *registryImpl) resolveAndLoad(image string) (ociutils.Manifest, []LoadResult, error) {
	name, tag, isDigest, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return ociutils.Manifest{}, nil, err
	}
	if !isDigest {
		return ociutils.Manifest{}, nil, errors.New("expected image with digest")
	}
	reg, err := remote.NewRepository(name)
	if err != nil {
		return ociutils.Manifest{}, nil, err
	}
	_, mfReader, err := reg.FetchReference(context.Background(), tag)
	if err != nil {
		return ociutils.Manifest{}, nil, err
	}
	defer funcutils.PanicOrLogOnErr(mfReader.Close, false, "failed to close reader")
	mf, err := ociutils.ParseManifestJSON(mfReader)
	if err != nil {
		return ociutils.Manifest{}, nil, err
	}
	// TODO: also support older manifest versions
	var artifacts []v1.Descriptor
	if len(mf.Layers) > 0 {
		artifacts = mf.Layers
	} else if len(mf.Blobs) > 0 {
		artifacts = mf.Blobs
	}

	res := make([]LoadResult, len(artifacts))
	for _, d := range artifacts {
		rc, err := reg.Fetch(context.Background(), d)
		if err != nil {
			return ociutils.Manifest{}, nil, err
		}
		fPath, err := r.ingest(d, rc)
		if err != nil {
			return ociutils.Manifest{}, nil, err
		}
		res = append(res, LoadResult{
			D:    d,
			Path: fPath,
		})
	}
	return *mf, res, nil
}

func (r *registryImpl) ResolveAndLoad(image string) (ociutils.Manifest, io.ReadCloser, error) {
	mf, res, err := r.resolveAndLoad(image)
	if err != nil {
		return ociutils.Manifest{}, nil, err
	}
	fp, err := os.Open(res[0].Path)
	if err != nil {
		return ociutils.Manifest{}, nil, err
	}
	return mf, fp, nil
}

func (r *registryImpl) ResolveAndLoadToPath(image, outputDir string) (ociutils.Manifest, []LoadResult, error) {
	mf, res, err := r.resolveAndLoad(image)
	if err != nil {
		return ociutils.Manifest{}, res, err
	}
	// make sure we have valid paths before moving any files around
	err = errors.Join(lo.Map(res, func(a LoadResult, _ int) error {
		p := a.D.Annotations[constants.OciImageTitle]
		if p == "" {
			return fmt.Errorf("empty path")
		}
		if path.IsAbs(p) {
			return fmt.Errorf("%q is absolute path", p)
		}
		return nil
	})...)
	if err != nil {
		return ociutils.Manifest{}, nil, err
	}
	// move files to the correct location
	for i, a := range res {
		p := a.D.Annotations[constants.OciImageTitle]
		targetPath := path.Join(outputDir, p)
		err = fileutils.ReplaceFile(a.Path, targetPath)
		if err != nil {
			return ociutils.Manifest{}, nil, err
		}
		// update path in-place within results slice
		res[i].Path = targetPath
	}
	return mf, res, nil
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
	// TODO: evaluate if we can use flocks to avoid conflicts when two instances want to fetch the same artifact
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
		if !errors.Is(err, errSeekNotSupported) {
			return "", err
		}
		// we start from 0 if we cannot seek
		n = 0
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
	err = fileutils.ReplaceFile(fPathDownload, fPathCompleted)
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
	return h, errSeekNotSupported
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
