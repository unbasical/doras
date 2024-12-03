package delta

import "io"

type Applier interface {
	Apply(old io.Reader, new io.Reader) (io.Reader, error)
}

type Creator interface {
	Create(old io.Reader, new io.Reader) (io.Reader, error)
}
