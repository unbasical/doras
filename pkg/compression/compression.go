package compression

import "io"

type Compressor interface {
	Compress(in io.Reader) (io.Reader, error)
}

type Decompressor interface {
	Decompress(in io.Reader) (io.Reader, error)
}
