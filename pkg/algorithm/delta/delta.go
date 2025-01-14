package delta

import (
	"github.com/unbasical/doras-server/pkg/algorithm"
	"io"
)

type Patcher interface {
	algorithm.Algorithm
	Patch(old io.Reader, patch io.Reader) (io.Reader, error)
}

type Differ interface {
	algorithm.Algorithm
	Diff(old io.Reader, new io.Reader) (io.ReadCloser, error)
}
