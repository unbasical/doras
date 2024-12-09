package delta

import "io"

type Patcher interface {
	Patch(old io.Reader, new io.Reader) (io.Reader, error)
}

type Differ interface {
	Diff(old io.Reader, new io.Reader) (io.Reader, error)
}
