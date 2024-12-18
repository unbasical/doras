package delta

import "io"

type Patcher interface {
	Patch(old io.Reader, patch io.Reader) (io.Reader, error)
	Name() string
}

type Differ interface {
	Diff(old io.Reader, new io.Reader) (io.ReadCloser, error)
	Name() string
}
