package compressionutils

import "io"

// CompressFunc abstracts over a compression function.
type CompressFunc func(reader io.ReadCloser) (io.ReadCloser, error)

// Compressor wraps around a compression function to implement the compression.Compressor interface.
type Compressor struct {
	Func CompressFunc
	Algo string
}

func (c *Compressor) Compress(in io.ReadCloser) (io.ReadCloser, error) {
	return c.Func(in)
}

func (c *Compressor) Name() string {
	return c.Algo
}

// DecompressorFunc abstracts over a compression function.
type DecompressorFunc func(reader io.Reader) (io.Reader, error)

// Decompressor wraps around a decompression function to implement the compression.Decompressor interface.
type Decompressor struct {
	Func DecompressorFunc
	Algo string
}

func (d *Decompressor) Decompress(input io.Reader) (io.Reader, error) {
	return d.Func(input)
}
func (d *Decompressor) Name() string {
	return d.Algo
}
