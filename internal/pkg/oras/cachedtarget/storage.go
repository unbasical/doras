package oci_file

import (
	"context"
	"errors"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/oras/ioutil"
	"io"
	"io/fs"
	"oras.land/oras-go/v2/content/file"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/errdef"
)

// bufPool is a pool of byte buffers that can be reused for copying content
// between files.
var bufPool = sync.Pool{
	New: func() interface{} {
		// the buffer size should be larger than or equal to 128 KiB
		// for performance considerations.
		// we choose 1 MiB here so there will be less disk I/O.
		buffer := make([]byte, 1<<20) // buffer size = 1 MiB
		return &buffer
	},
}

// Storage is a CAS based on file system with the OCI-Image layout.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0/image-layout.md
type Storage struct {
	*ReadOnlyStorage
	// cacheRoot is the cacheRoot directory of the OCI layout.
	cacheRoot string
	// ingestRoot is the cacheRoot directory of the temporary ingest files.
	ingestRoot string
	// fsRoot
	fsRoot                    string
	DisableOverwrite          bool
	AllowPathTraversalOnWrite bool
	SymlinkFromCache          bool
}

// NewStorage creates a new CAS based on file system with the OCI-Image layout.
func NewStorage(root, fsRoot string, symlinkFromCache bool) (*Storage, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", root, err)
	}
	fsRootAbs, err := filepath.Abs(fsRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", fsRoot, err)
	}
	err = ensureDir(fsRootAbs)
	if err != nil {
		return nil, fmt.Errorf("failed to create directories for %s: %w", fsRootAbs, err)
	}

	return &Storage{
		ReadOnlyStorage:  NewStorageFromFS(os.DirFS(rootAbs)),
		cacheRoot:        rootAbs,
		fsRoot:           fsRootAbs,
		ingestRoot:       filepath.Join(rootAbs, "ingest"),
		SymlinkFromCache: symlinkFromCache,
	}, nil
}

// Push pushes the content, matching the expected descriptor.
func (s *Storage) Push(_ context.Context, expected ocispec.Descriptor, content io.Reader) error {
	path, err := BlobPath(expected.Digest)
	if err != nil {
		return fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrInvalidDigest)
	}
	target := filepath.Join(s.cacheRoot, path)

	// check if the target content already exists in the blob directory.
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := ensureDir(filepath.Dir(target)); err != nil {
		return err
	}

	// write the content to a temporary ingest file.
	ingest, err := s.ingest(expected, content)
	if err != nil {
		return err
	}

	// move the content from the temporary ingest file to the target path.
	// since blobs are read-only once stored, if the target blob already exists,
	// Rename() will fail for permission denied when trying to overwrite it.
	if err := os.Rename(ingest, target); err != nil {
		// remove the ingest file in case of error
		os.Remove(ingest)
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
		}

		return err
	}

	if name := expected.Annotations[ocispec.AnnotationTitle]; name != "" {
		outputPath, err := s.resolveWritePath(name)
		if err != nil {
			panic(err)
		}

		if !s.SymlinkFromCache {
			// move file to output dir
			// update permissions
			// create symlink to cache
			if err := os.Rename(target, outputPath); err != nil {
				panic(err)
			}
			if err := os.Chmod(outputPath, os.FileMode(0600)); err != nil {
				panic(err)
			}
			if err := os.Symlink(outputPath, target); err != nil {
				panic(err)
			}
		} else {
			err = os.Symlink(target, outputPath)
			if err != nil {
				panic(err)
			}
		}
	}
	return nil
}

// Delete removes the target from the system.
func (s *Storage) Delete(ctx context.Context, target ocispec.Descriptor) error {
	path, err := BlobPath(target.Digest)
	if err != nil {
		return fmt.Errorf("%s: %s: %w", target.Digest, target.MediaType, errdef.ErrInvalidDigest)
	}
	targetPath := filepath.Join(s.cacheRoot, path)
	err = os.Remove(targetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s: %s: %w", target.Digest, target.MediaType, errdef.ErrNotFound)
		}
		return err
	}
	return nil
}

func (s *Storage) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	exists, err := s.ReadOnlyStorage.Exists(ctx, target)
	if !exists {
		return false, err
	}
	if name := target.Annotations[ocispec.AnnotationTitle]; name != "" {
		path, err := s.resolveWritePath(name)
		if err != nil {
			return false, err
		}
		_, err = os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		fmt.Printf("%s exists\n", target.Digest)
		return true, err
	}
	return true, err
}

// ingest write the content into a temporary ingest file.
func (s *Storage) ingest(expected ocispec.Descriptor, content io.Reader) (path string, ingestErr error) {
	if err := ensureDir(s.ingestRoot); err != nil {
		return "", fmt.Errorf("failed to ensure ingest dir: %w", err)
	}
	var writer *ioutil.SkipWriter
	// use reflection magic to pick up partially downloaded ingest files
	// todo refactor this to use nicer pattern
	if reflect.TypeOf(content).Implements(reflect.TypeOf((*io.ReadSeeker)(nil)).Elem()) {
		entries, err := os.ReadDir(s.ingestRoot)
		if err != nil {
			panic(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), expected.Digest.Encoded()) {
				// open the previously downloaded file for reading
				fpR, err := os.Open(filepath.Join(s.ingestRoot, entry.Name()))
				if err != nil {
					return "", fmt.Errorf("failed to open file %s for download continuation: %w", entry.Name(), err)
				}
				// get file size
				fstat, err := fpR.Stat()
				if err != nil {
					panic(err)
				}
				size := fstat.Size()

				path = fpR.Name()

				// also open the previous file for writing
				fpW, err := os.OpenFile(filepath.Join(s.ingestRoot, entry.Name()), os.O_WRONLY, 0600)
				if err != nil {
					panic(err)
				}
				defer fpW.Close()

				_, err = fpW.Seek(size, io.SeekStart)
				if err != nil {
					panic(err)
				}
				// adjust content reader so it continues from where we left off
				_, err = content.(io.Seeker).Seek(size, io.SeekStart)
				if err != nil {
					panic(err)
				}
				// create combine writer that reads the previously downloaded bytes and then fetches the rest
				// the LimitReader is necessary so we do not keep reading the file as we write to it
				content = io.MultiReader(io.LimitReader(fpR, size), content)
				// the copying functionality needs to process the entire file to accept it
				// to avoid writing content multiple times to the same file we need to skip some of the stream
				// provided by the reader
				w := ioutil.NewSkipWriter(fpW, int(size))
				writer = &w
				// break is necessary in case there are multiple ingest files for the same blob
				fmt.Printf("continuing from %d/%d bytes\n", size, expected.Size)
				break
			}
		}

	}
	if writer == nil {
		fp, err := os.CreateTemp(s.ingestRoot, expected.Digest.Encoded()+"_*")
		if err != nil {
			return "", fmt.Errorf("failed to create ingest file: %w", err)
		}
		path = fp.Name()
		defer func() {
			// close the temp file and check close error
			if err := fp.Close(); err != nil && ingestErr == nil {
				ingestErr = fmt.Errorf("failed to close ingest file: %w", err)
			}

			// remove the temp file in case of error
			if ingestErr != nil {
				os.Remove(path)
			}
		}()
		w := ioutil.NewSkipWriter(fp, 0)
		writer = &w
	}

	// create a temp file with the file name format "blobDigest_randomString"
	// in the ingest directory.
	// Go ensures that multiple programs or goroutines calling CreateTemp
	// simultaneously will not choose the same file.

	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	if err := ioutil.CopyBuffer(writer, content, *buf, expected); err != nil {
		fmt.Printf("fetching failed after reading %d new bytes out of %d in total\n", writer.BytesWritten(), expected.Size)
		return "", fmt.Errorf("failed to ingest: %w", err)
	}
	fmt.Printf("successfully fetched %d new bytes out of %d\n", writer.BytesWritten(), expected.Size)

	// change to readonly
	if err := os.Chmod(path, 0444); err != nil {
		return "", fmt.Errorf("failed to make readonly: %w", err)
	}

	return
}

// resolveWritePath resolves the path to write for the given name.
func (s *Storage) resolveWritePath(name string) (string, error) {
	path := s.absPath(name)
	if !s.AllowPathTraversalOnWrite {
		base, err := filepath.Abs(s.fsRoot)
		if err != nil {
			return "", err
		}
		target, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(base, target)
		if err != nil {
			return "", file.ErrPathTraversalDisallowed
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			return "", file.ErrPathTraversalDisallowed
		}
	}
	if s.DisableOverwrite {
		if _, err := os.Stat(path); err == nil {
			return "", file.ErrOverwriteDisallowed
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	return path, nil
}

// ensureDir ensures the directories of the path exists.
func ensureDir(path string) error {
	return os.MkdirAll(path, 0777)
}

// absPath returns the absolute path of the path.
func (s *Storage) absPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.fsRoot, path)
}
