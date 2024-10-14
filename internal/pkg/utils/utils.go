package utils

import (
	"crypto/sha256"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"path/filepath"
)

func CalcSha256Hex(b []byte) string {
	hash := sha256.Sum256(b)
	return fmt.Sprintf("%x", hash)
}

// VerifyPath adapted from: https://www.stackhawk.com/blog/golang-path-traversal-guide-examples-and-prevention/
func VerifyPath(path, trustedRoot string, mustExist bool) (string, error) {
	log.Debugf("verifying path `%s` using `%s` as trustedRoot", path, trustedRoot)
	c := filepath.Clean(path)
	log.Debug("cleaned path: " + c)
	r, err := filepath.EvalSymlinks(c)
	if err != nil && !mustExist {
		// we cannot evaluate the symlink if the file does not exist
		r = c
	} else if err != nil {
		log.Debug("Error " + err.Error())
		return c, errors.New("unsafe or invalid path specified")
	}
	log.Debugf("EvalSymlinks() == %s", r)
	err = inTrustedRoot(r, trustedRoot)
	log.Debug(err)
	if err != nil {
		log.Debugf("path `%s` not in trusted root: %s", r, err)
		return r, errors.New("unsafe or invalid path specified")
	} else {
		log.Debugf("provided path `%s` passed checks", r)
		return r, nil
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
