package delta

import (
	"github.com/unbasical/doras-server/pkg/algorithm"
	"io"
)

// Patcher abstracts over the delta application aspect of a diffing algorithm.
type Patcher interface {
	algorithm.Algorithm
	// Patch returns a reader that applies the given patch to the input.
	Patch(old io.Reader, patch io.Reader) (io.Reader, error)
}

// Differ abstracts over the delta creation aspect of a diffing algorithm.
type Differ interface {
	algorithm.Algorithm
	// Diff creates a patch from old to new.
	Diff(old io.Reader, new io.Reader) (io.ReadCloser, error)
}
