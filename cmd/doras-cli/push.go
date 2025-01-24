package main

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/compression/gzip"
	"github.com/unbasical/doras/internal/pkg/compression/zstd"
	"github.com/unbasical/doras/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras/pkg/algorithm/compression"
	"io"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
	"os"
	"path/filepath"
	"time"
)

// push image to the registry
func (args *cliArgs) push(ctx context.Context) error {
	repoName, tag, isDigest, err := ociutils.ParseOciImageString(args.Push.Image)
	if err != nil {
		return err
	}
	if isDigest {
		return errors.New("cannot push digest image")
	}
	// init repo (credentials etc.)
	repo, err := args.setupRepo(repoName)
	if err != nil {
		return err
	}

	// are we pushing a file or a directory?
	exists, isDir, err := fileutils.ExistsAndIsDirectory(args.Push.Path)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("path %q does not exist", args.Push.Path)
	}

	switch isDir {
	case true:
		err = pushDirectory(ctx, args, repo, tag)
	case false:
		err = pushFile(ctx, args, repo, tag)
	}
	if err != nil {
		return err
	}
	return nil
}

// pushFile to the registry
func pushFile(ctx context.Context, args *cliArgs, target oras.Target, tag string) error {
	mediaType := "application/vnd.oci.image.layer.v1.tar"
	fName := args.Push.Path
	fp, err := os.Open(args.Push.Path)
	if err != nil {
		return err
	}
	compressor, err := args.getCompressor()
	if err != nil {
		return err
	}
	if compressor.Name() != "" {
		compressedFile, err := compressor.Compress(fp)
		if err != nil {
			return err
		}
		tmpDir := os.TempDir()
		tempFile, err := os.CreateTemp(tmpDir, "doras_push_tmp*."+compressor.Name())
		if err != nil {
			return err
		}
		mediaType = fmt.Sprintf("%s+%s", mediaType, compressor.Name())
		fName = fmt.Sprintf("%s.%s", fName, compressor.Name())
		log.Info("Writing compressed file to temp file.")
		_, err = io.Copy(tempFile, compressedFile)
		if err != nil {
			return err
		}

		defer funcutils.PanicOrLogOnErr(fp.Close, false, "failed to close file")
		defer func() {
			log.Info("Cleaning up temp file.")
			_ = os.Remove(tempFile.Name())
		}()
		fp = tempFile
		// Reset cursor before hashing file.
		resetFileOrPanic(fp)
	}
	stat, err := fp.Stat()
	if err != nil {
		return err
	}
	dgst, err := digest.FromReader(fp)
	if err != nil {
		return err
	}

	d := v1.Descriptor{
		MediaType: mediaType,
		Digest:    dgst,
		Size:      stat.Size(),
		Annotations: map[string]string{
			"org.opencontainers.image.title": fName,
		},
	}
	log.Debugf("artifact descriptor: %v", d)
	// Reset cursor before pushing file.
	resetFileOrPanic(fp)

	_, err = pushAndTag(ctx, target, d, fp, tag)
	if err != nil {
		return err
	}
	return nil
}

func resetFileOrPanic(fp *os.File) {
	_, err := fp.Seek(0, io.SeekStart)
	if err != nil {
		panic(err)
	}
}

// pushDirectory to the registry as an optionally compressed tar archive
func pushDirectory(ctx context.Context, args *cliArgs, target oras.Target, tag string) error {
	mediaType := "application/vnd.oci.image.layer.v1.tar"
	dirName := args.Push.Path

	tmpDir := os.TempDir()
	tempFile, err := os.CreateTemp(tmpDir, "doras_push_tmp*")
	if err != nil {
		return err
	}
	defer func() {
		log.Info("Cleaning up temp file.")
		_ = os.Remove(tempFile.Name())
	}()
	compressor, err := args.getCompressor()
	if err != nil {
		return err
	}

	// turn writer into reader
	pr, pw := io.Pipe()
	go func() {
		err := tarDirectory(ctx, dirName, dirName, pw, true)
		if err != nil {
			log.WithError(err).Error("failed to add FS to tar")
			_ = pw.CloseWithError(err)
		}
		_ = pw.Close()
	}()

	// Compress and hash file while writing it to the disk.
	compressedFile, err := compressor.Compress(pr)
	if err != nil {
		return err
	}
	if compressor.Name() != "" {
		mediaType = fmt.Sprintf("%s+%s", mediaType, compressor.Name())
	}
	hasher := sha256.New()
	hashedCompressedFile := io.TeeReader(compressedFile, hasher)
	log.Debugf("attempting to create compressed tar archive at %q", tempFile.Name())
	_, err = io.Copy(tempFile, hashedCompressedFile)
	if err != nil {
		return err
	}
	// Get metadata.
	stat, err := tempFile.Stat()
	if err != nil {
		return err
	}
	dgst := digest.NewDigest("sha256", hasher)

	// Set metadata.
	d := v1.Descriptor{
		MediaType: mediaType,
		Digest:    dgst,
		Size:      stat.Size(),
		Annotations: map[string]string{
			"org.opencontainers.image.title": dirName,
			"io.deis.oras.content.unpack":    "true",
		},
	}
	log.Debugf("artifact descriptor: %v", d)
	// Reset cursor before pushing file.
	resetFileOrPanic(tempFile)

	mfDesc, err2 := pushAndTag(ctx, target, d, tempFile, tag)
	if err2 != nil {
		return err2
	}
	log.Infof("successfully uploaded (%d bytes) artifact to: %q (digest: %s)", stat.Size(), args.Push.Image, mfDesc.Digest.String())
	return nil
}

// pushAndTag push file and its manifest to the registry and tag it.
func pushAndTag(ctx context.Context, target oras.Target, d v1.Descriptor, content io.Reader, tag string) (v1.Descriptor, error) {
	// Push blob to the registry.
	err := target.Push(ctx, d, content)
	if err != nil {
		return v1.Descriptor{}, err
	}

	// Create the corresponding manifest and push it.
	artifactType := "application/vnd.test.artifact"
	opts := oras.PackManifestOptions{
		Layers: []v1.Descriptor{d},
	}
	mfDesc, err := oras.PackManifest(ctx, target, oras.PackManifestVersion1_1, artifactType, opts)
	if err != nil {
		return v1.Descriptor{}, err
	}
	log.Infof("manifest descriptor: %v", mfDesc)
	// Tag the image.
	if err = target.Tag(ctx, mfDesc, tag); err != nil {
		return v1.Descriptor{}, fmt.Errorf("failed to tag manifest: %v", err)
	}
	return mfDesc, nil
}

// getCompressor loads the correct compression.Compressor from the set algorithm.
func (args *cliArgs) getCompressor() (compression.Compressor, error) {
	switch args.Push.Compress {
	case "zstd":
		return zstd.NewCompressor(), nil
	case "gzip":
		return gzip.NewCompressor(), nil
	case "none":
		return compressionutils.NewNopCompressor(), nil
	default:
		return nil, fmt.Errorf("unknown compression algorithm: %q", args.Push.Compress)
	}
}

// setupRepo with path, HTTP client, credentials etc.
func (args *cliArgs) setupRepo(repoName string) (oras.Target, error) {
	// Set up repository.
	repo, err := remote.NewRepository(repoName)
	if err != nil {
		return nil, err
	}
	// Set up repo credentials.
	credentials, err := args.getCredentialFunc()
	if err != nil {
		return nil, err
	}
	if args.InsecureAllowHTTP {
		log.Warn("INSECURE REGISTRY CONNECTIONS ARE ENABLED, ONLY USE THIS FOR LOCAL DEVELOPMENT")
		repo.PlainHTTP = true
	}
	repo.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials,
	}
	return repo, nil
}

// tarDirectory walks the directory specified by path, and tar those files with a new
// path prefix.
//
//nolint:revive // This rule is disabled to get around complexity linter errors. It is a slightly modified copy from the oras-go lib https://github.com/oras-project/oras-go/blob/dff56286a744d805bf953ada296e6076c335258b/content/file/utils.go#L36C92-L112.
func tarDirectory(ctx context.Context, root, prefix string, w io.Writer, removeTimes bool) (err error) {
	tw := tar.NewWriter(w)
	defer func() {
		closeErr := tw.Close()
		if err == nil {
			err = closeErr
		}
	}()

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) (returnErr error) {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Rename path
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name = filepath.Join(prefix, name)
		name = filepath.ToSlash(name)

		// Generate header
		// NOTE: We don't support hard links and treat it as regular files
		var link string
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		header.Name = name
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""

		if removeTimes {
			header.ModTime = time.Time{}
			header.AccessTime = time.Time{}
			header.ChangeTime = time.Time{}
		}

		// Write file
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if mode.IsRegular() {
			fp, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() {
				closeErr := fp.Close()
				if returnErr == nil {
					returnErr = closeErr
				}
			}()

			if _, err := io.Copy(tw, fp); err != nil {
				return fmt.Errorf("failed to copy to %s: %w", path, err)
			}
		}

		return nil
	})
}
