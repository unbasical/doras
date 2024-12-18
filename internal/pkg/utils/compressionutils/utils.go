package compressionutils

import "io"

type CompressFunc func(reader io.ReadCloser) (io.ReadCloser, error)

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

type DecompressorFunc func(reader io.Reader) (io.Reader, error)
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
