package pathsanitize

import (
	"errors"
	"github.com/sirupsen/logrus"
	"path/filepath"
)

// VerifyPath adapted from: https://www.stackhawk.com/blog/golang-path-traversal-guide-examples-and-prevention/
func VerifyPath(path, trustedRoot string) (string, error) {
	logrus.Debugf("verifying path `%s` using `%s` as trustedRoot", path, trustedRoot)
	c := filepath.Clean(path)
	err := inTrustedRoot(c, trustedRoot)
	logrus.Debug(err)
	if err != nil {
		logrus.Debugf("path `%s` not in trusted root: %s", c, err)
		return c, errors.New("unsafe or invalid path specified: " + err.Error())
	} else {
		logrus.Debugf("provided path `%s` passed checks", c)
		return c, nil
	}
}
func inTrustedRoot(path, trustedRoot string) error {
	// this can lead to an infinite loop if path never becomes equal to "/" or trustedRoot
	for path != "/" {
		logrus.Debug(path)
		path = filepath.Dir(path)
		if path == trustedRoot {
			return nil
		}
	}
	return errors.New("path is outside of trusted root")
}
