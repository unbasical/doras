package fileutils

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"io"
	"os"
	"path/filepath"

	"github.com/unbasical/doras/internal/pkg/utils/writerutils"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// SafeReadJSON reads the JSON file at the path into the targetPointer.
// Returns true if the file exists or an error if an error occurred.
func SafeReadJSON(filePath string, targetPointer any, perm os.FileMode) (jsonAvailable bool, err error) {
	fileBytes, err := SafeReadFile(filePath, perm)
	if err != nil {
		return false, fmt.Errorf("unable to open file: %s, %w", filePath, err)
	}

	if len(fileBytes) == 0 {
		return false, nil
	}
	return true, json.Unmarshal(fileBytes, targetPointer)
}

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

// SafeWriteJson writes the provided object to a JSON file at the provided path.
// The function makes sure any changes are flushed to the disk before returning.
func SafeWriteJson[T any](filePath string, targetPointer *T) error {
	fp, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	w := writerutils.NewSafeFileWriter(fp)
	defer funcutils.PanicOrLogOnErr(w.Close, true, "failed to close writer")
	err = json.NewEncoder(w).Encode(*targetPointer)
	if err != nil {
		return err
	}
	return nil
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
//
//nolint:revive // Disable complexity warning, this function should be understandable enough to people familiar with navigating trees.
func CompareDirectories(dir1, dir2 string) (bool, error) {
	files1 := make(map[string][32]byte)

	// Walk through dir1 and store file hashes
	err := filepath.Walk(dir1, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(dir1, path)
		if info.IsDir() {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		files1[relPath] = hash
		return nil
	})
	if err != nil {
		return false, err
	}

	// Walk through dir2 and compare with files1
	err = filepath.Walk(dir2, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(dir2, path)
		if info.IsDir() {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			return err
		}

		if hash1, exists := files1[relPath]; !exists || hash1 != hash {
			return fmt.Errorf("file mismatch: %s", relPath)
		}

		delete(files1, relPath)
		return nil
	})
	if err != nil {
		return false, err
	}

	// If files1 is not empty, it means dir1 had extra files
	if len(files1) > 0 {
		return false, fmt.Errorf("directory contains %d extra files", len(files1))
	}

	return true, nil
}

// hashFile computes a SHA-256 hash of the file content
func hashFile(path string) ([32]byte, error) {
	var hash [32]byte
	file, err := os.Open(path)
	if err != nil {
		return hash, err
	}
	defer func() {
		_ = file.Close()
	}()

	hasher := sha256.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return hash, err
	}

	copy(hash[:], hasher.Sum(nil))
	return hash, nil
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
