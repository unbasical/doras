package compressionutils

import (
	"io"

	"github.com/unbasical/doras/pkg/algorithm/compression"
)

type noCompression struct {
}

// NewNopCompressor returns a null compression.Compressor that does no compression at all.
func NewNopCompressor() compression.Compressor {
	return &noCompression{}
}

// NewNopDecompressor returns a null compression.Decompressor that does no decompression at all.
func NewNopDecompressor() compression.Decompressor {
	return &noCompression{}
}

func (n noCompression) Decompress(in io.Reader) (io.Reader, error) {
	// Do nothing.
	return in, nil
}

func (n noCompression) Compress(in io.ReadCloser) (io.ReadCloser, error) {
	// Do nothing.
	return in, nil
}

func (n noCompression) Name() string {
	// There is no name.
	return ""
}
