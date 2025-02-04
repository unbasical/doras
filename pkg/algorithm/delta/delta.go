package delta

import (
	"github.com/opencontainers/go-digest"
	"github.com/unbasical/doras/pkg/algorithm"
	"io"
)

// Patcher abstracts over the delta application aspect of a diffing algorithm.
type Patcher interface {
	algorithm.Algorithm
	// Patch returns a reader that applies the given patch to the input.
	Patch(old io.Reader, patch io.Reader) (io.Reader, error)
	PatchFilesystem(artifactPath string, patch io.Reader, expected *digest.Digest) error
}

// Differ abstracts over the delta creation aspect of a diffing algorithm.
type Differ interface {
	algorithm.Algorithm
	// Diff creates a patch from old to new.
	Diff(oldfile io.Reader, newfile io.Reader) (io.ReadCloser, error)
}
