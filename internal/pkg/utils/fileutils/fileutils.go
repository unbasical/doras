package fileutils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/unbasical/doras-server/internal/pkg/utils/writerutils"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

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
func SafeReadYAML(filePath string, targetPointer any, perm os.FileMode) (jsonAvailable bool, err error) {
	fileBytes, err := SafeReadFile(filePath, perm)
	if err != nil {
		return false, fmt.Errorf("unable to open file: %s, %w", filePath, err)
	}

	if len(fileBytes) == 0 {
		return false, nil
	}
	return true, yaml.Unmarshal(fileBytes, targetPointer)
}

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
func SafeWriteJson[T any](filePath string, targetPointer *T) error {
	fp, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	w := writerutils.NewSafeFileWriter(fp)
	defer w.Close()
	err = json.NewEncoder(w).Encode(*targetPointer)
	if err != nil {
		return err
	}
	return nil
}

func ReadOrPanic(p string) []byte {
	data, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	return data
}
