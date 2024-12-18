package compression

import "io"

type Compressor interface {
	Compress(in io.ReadCloser) (io.ReadCloser, error)
	Name() string
}

type Decompressor interface {
	Decompress(in io.Reader) (io.Reader, error)
	Name() string
}
