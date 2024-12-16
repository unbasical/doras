package compressionutils

import (
	"io"

	"github.com/unbasical/doras-server/pkg/compression"
)

type noCompression struct {
}

func NewNopCompressor() compression.Compressor {
	return &noCompression{}
}

func NewNopDecompressor() compression.Compressor {
	return &noCompression{}
}

func (n noCompression) Decompress(in io.Reader) (io.Reader, error) {
	return in, nil
}

func (n noCompression) Compress(in io.Reader) (io.Reader, error) {
	return in, nil
}

func (n noCompression) Name() string {
	return ""
}
