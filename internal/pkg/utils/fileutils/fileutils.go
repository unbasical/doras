package fileutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"io"
	"os"

	"github.com/unbasical/doras-server/internal/pkg/utils/writerutils"

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
