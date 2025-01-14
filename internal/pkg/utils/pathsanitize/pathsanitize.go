package pathsanitize

import (
	"errors"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// VerifyPath adapted from: https://www.stackhawk.com/blog/golang-path-traversal-guide-examples-and-prevention/
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
