package compressionutils

import "io"

type CompressorFunc func(reader io.Reader) (io.Reader, error)

func (fn CompressorFunc) Compress(input io.Reader) (io.Reader, error) {
	return fn(input)
}

type DecompressorFunc func(reader io.Reader) (io.Reader, error)

func (fn CompressorFunc) Decompress(input io.Reader) (io.Reader, error) {
	return fn(input)
}
