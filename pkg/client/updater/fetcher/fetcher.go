package fetcher

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras/internal/pkg/utils/writerutils"
	"github.com/unbasical/doras/pkg/constants"
	"hash"
	"io"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
	"os"
	"path"
	"strings"
)

// ArtifactLoader is used to load artifacts from a registry.
// This should handle partial downloads if possible.
type ArtifactLoader interface {
	ResolveAndLoadToPath(image, outputDir string) (v1.Descriptor, ociutils.Manifest, []LoadResult, error)
}

type registryImpl struct {
	workingDir string
	StorageSource
}

var errSeekNotSupported = errors.New("reader does not support seeking")

// LoadResult represents the result of a load. A layer with v1.Descriptor D is stored at Path.
type LoadResult struct {
	D    v1.Descriptor
	Path string
}

// StorageSource is used to get an oras.ReadOnlyTarget for an image string.
type StorageSource interface {
	GetTarget(repoName string) (oras.ReadOnlyTarget, error)
}

type repoStorageSource struct {
	auth.CredentialFunc
	InsecureAllowHttp bool
}

// NewRepoStorageSource implements StorageSource for remote registries.
func NewRepoStorageSource(insecureAllowHttp bool, credentialFunc auth.CredentialFunc) StorageSource {
	return &repoStorageSource{
		InsecureAllowHttp: insecureAllowHttp,
		CredentialFunc:    credentialFunc,
	}
}

// NewArtifactLoader returns an artifact loader that fetches artifacts via the provided StorageSource.
// It implements the ability to pick up interrupted fetches.
func NewArtifactLoader(workingDir string, storageSource StorageSource) ArtifactLoader {
	return &registryImpl{
		workingDir:    workingDir,
		StorageSource: storageSource,
	}
}

func (r *repoStorageSource) GetTarget(repoName string) (oras.ReadOnlyTarget, error) {
	reg, err := remote.NewRepository(repoName)
	if err != nil {
		return nil, err
	}
	reg.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Credential: r.CredentialFunc,
	}
	reg.PlainHTTP = r.InsecureAllowHttp
	return reg, nil
}

func (r *registryImpl) resolveAndLoad(image string) (v1.Descriptor, ociutils.Manifest, []LoadResult, error) {
	ctx := context.Background()
	name, tag, _, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return v1.Descriptor{}, ociutils.Manifest{}, nil, err
	}
	tag = strings.TrimPrefix(tag, "@")
	src, err := r.StorageSource.GetTarget(name)
	if err != nil {
		return v1.Descriptor{}, ociutils.Manifest{}, nil, err
	}
	mfD, err := src.Resolve(ctx, tag)
	if err != nil {
		return v1.Descriptor{}, ociutils.Manifest{}, nil, err
	}
	mfReader, err := src.Fetch(ctx, mfD)
	if err != nil {
		return v1.Descriptor{}, ociutils.Manifest{}, nil, err
	}
	defer funcutils.PanicOrLogOnErr(mfReader.Close, false, "failed to close reader")
	mf, err := ociutils.ParseManifestJSON(mfReader)
	if err != nil {
		return v1.Descriptor{}, ociutils.Manifest{}, nil, err
	}
	// TODO: also support older manifest versions
	var artifacts []v1.Descriptor
	if len(mf.Layers) > 0 {
		artifacts = mf.Layers
	} else if len(mf.Blobs) > 0 {
		artifacts = mf.Blobs
	}

	res := make([]LoadResult, 0, len(artifacts))
	for _, d := range artifacts {
		rc, err := src.Fetch(context.Background(), d)
		if err != nil {
			return v1.Descriptor{}, ociutils.Manifest{}, nil, err
		}
		fPath, err := r.ingest(d, rc)
		if err != nil {
			return v1.Descriptor{}, ociutils.Manifest{}, nil, err
		}
		res = append(res, LoadResult{
			D:    d,
			Path: fPath,
		})
	}
	return mfD, *mf, res, nil
}

func (r *registryImpl) ResolveAndLoadToPath(image, outputDir string) (v1.Descriptor, ociutils.Manifest, []LoadResult, error) {
	mfD, mf, res, err := r.resolveAndLoad(image)
	if err != nil {
		return v1.Descriptor{}, ociutils.Manifest{}, nil, err
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
		return v1.Descriptor{}, ociutils.Manifest{}, nil, err
	}
	// move files to the correct location
	for i, a := range res {
		p := a.D.Annotations[constants.OciImageTitle]

		log.Debugf("%q", p)
		targetPath := path.Join(outputDir, p)
		if targetPath == outputDir {
			targetPath = path.Join(targetPath, "archive")
		}
		log.Debugf("attempting to move to %q", targetPath)
		err = fileutils.ReplaceFile(a.Path, targetPath)
		if err != nil {
			return v1.Descriptor{}, ociutils.Manifest{}, nil, err
		}
		// update path in-place within results slice
		res[i].Path = targetPath
	}
	return mfD, mf, res, nil
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
