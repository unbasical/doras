package fileutils

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/gofrs/flock"
	"golang.org/x/mod/sumdb/dirhash"
	"hash"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// SafeReadYAML reads the YAML file at the path into the targetPointer.
// Returns true if the file exists or an error if an error occurred.
func SafeReadYAML(filePath string, targetPointer any, perm os.FileMode) (yamlAvailable bool, err error) {
	fileBytes, err := SafeReadFile(filePath, perm)
	if err != nil {
		return false, fmt.Errorf("unable to open file: %s, %w", filePath, err)
	}

	if len(fileBytes) == 0 {
		return false, nil
	}
	return true, yaml.Unmarshal(fileBytes, targetPointer)
}

// SafeReadFile reads the file at the provided path into a byte slice.
func SafeReadFile(filePath string, perm os.FileMode) ([]byte, error) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, perm)
	if err != nil {
		return nil, fmt.Errorf("unable to open file: %s, %w", filePath, err)
	}

	bytes, readErr := io.ReadAll(file)
	if err = file.Close(); err != nil {
		logrus.Errorf("Failed to close file: %s", filePath)
	}
	return bytes, readErr
}

// ReadOrPanic reads the entire file at the provided path or panics if it is not possible.
func ReadOrPanic(p string) []byte {
	data, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	return data
}

func ExistsAndIsDirectory(path string) (exists, isDir bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, false, nil
		}
		return false, false, err
	}
	return true, info.IsDir(), nil
}

// CompareDirectories checks if two directories have the same structure and content.
// Walks both folders and ensures the contents are identical (compares file hashes).
func CompareDirectories(dir1, dir2 string) (bool, error) {
	return CompareDirectoriesHasher(dir1, dir2, sha256.New())
}

// CompareDirectoriesHasher checks if two directories have the same structure and content.
// Walks both folders and ensures the contents are identical (compares file hashes).
func CompareDirectoriesHasher(dir1, dir2 string, h hash.Hash) (bool, error) {
	dirHash, err := dirhash.HashDir(dir1, "", getDirHasher(cloneHash(h)))
	if err != nil {
		return false, err
	}
	dirHash2, err := dirhash.HashDir(dir2, "", getDirHasher(cloneHash(h)))
	if err != nil {
		return false, err
	}
	return dirHash == dirHash2, nil
}

// cloneHash creates a new instance of the underlying hash type by using reflection.
// It assumes that proto is a pointer to a struct.
func cloneHash(h hash.Hash) hash.Hash {
	v := reflect.ValueOf(h)
	if v.Kind() != reflect.Ptr {
		panic("provided hash is not a pointer")
	}
	newInstance := reflect.New(v.Elem().Type()).Interface().(hash.Hash)
	return newInstance
}

// CleanDirectory removes all files and subdirectories within dirPath,
// leaving the directory itself intact.
func CleanDirectory(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return err
		}
	}
	return nil
}

func getDirHasher(h hash.Hash) func([]string, func(string) (io.ReadCloser, error)) (string, error) {
	return func(files []string, open func(string) (io.ReadCloser, error)) (string, error) {
		files = append([]string(nil), files...)
		sort.Strings(files)
		for _, file := range files {
			if strings.Contains(file, "\n") {
				return "", errors.New("dirhash: filenames with newlines are not supported")
			}
			r, err := open(file)
			if err != nil {
				return "", err
			}
			hf := sha256.New()
			_, err = io.Copy(hf, r)
			_ = r.Close()
			if err != nil {
				return "", err
			}
			_, _ = fmt.Fprintf(h, "%x  %s\n", hf.Sum(nil), file)
		}
		return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
	}
}

// LockedFile wraps an *os.File and a file lock using gofrs/flock.
type LockedFile struct {
	f    *os.File
	lock *flock.Flock
}

// OpenLockedFile opens the file with the specified flags and permissions,
// acquires an exclusive lock (blocking until it can be obtained), and returns
// a LockedFile. The lock file is created by appending ".lock" to the filename.
func OpenLockedFile(path string, flag int, perm os.FileMode) (*LockedFile, error) {
	// Create a flock instance for the lock file.
	// Using a separate lock file (e.g., "example.txt.lock") is a common pattern.
	lock := flock.New(path + ".lock")
	// Open the target file.
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}

	// Acquire an exclusive lock. This call blocks until the lock is acquired.
	if err = lock.Lock(); err != nil {
		return nil, errors.Join(err, f.Close())
	}

	return &LockedFile{
		f:    f,
		lock: lock,
	}, nil
}

// Stat returns file metadata by delegating to the underlying os.File.
func (lf *LockedFile) Stat() (os.FileInfo, error) {
	return lf.f.Stat()
}

func (lf *LockedFile) Read(p []byte) (int, error) {
	return lf.f.Read(p)
}

func (lf *LockedFile) Write(p []byte) (int, error) {
	return lf.f.Write(p)
}

// Close releases the lock and then closes the underlying file.
func (lf *LockedFile) Close() error {
	// First, release the lock.
	if err := lf.lock.Unlock(); err != nil {
		return err
	}
	// Then close the file.
	return errors.Join(
		lf.f.Sync(),
		lf.f.Close(),
	)
}
