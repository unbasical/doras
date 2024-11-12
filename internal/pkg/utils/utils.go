package utils

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

func CalcSha256Hex(b []byte) string {
	hash := sha256.Sum256(b)
	return fmt.Sprintf("%x", hash)
}

// VerifyPath adapted from: https://www.stackhawk.com/blog/golang-path-traversal-guide-examples-and-prevention/
func VerifyPath(path, trustedRoot string) (string, error) {
	log.Debugf("verifying path `%s` using `%s` as trustedRoot", path, trustedRoot)
	c := filepath.Clean(path)
	err := inTrustedRoot(c, trustedRoot)
	log.Debug(err)
	if err != nil {
		log.Debugf("path `%s` not in trusted root: %s", c, err)
		return c, errors.New("unsafe or invalid path specified: " + err.Error())
	} else {
		log.Debugf("provided path `%s` passed checks", c)
		return c, nil
	}
}
func inTrustedRoot(path, trustedRoot string) error {
	// this can lead to an infinite loop if path never becomes equal to "/" or trustedRoot
	for path != "/" {
		log.Debug(path)
		path = filepath.Dir(path)
		if path == trustedRoot {
			return nil
		}
	}
	return errors.New("path is outside of trusted root")
}

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
		log.Errorf("Failed to close file: %s", filePath)
	}
	return bytes, readErr
}
