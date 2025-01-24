package compression

import (
	"github.com/unbasical/doras/pkg/algorithm"
	"io"
)

// Compressor is used to abstract over the compression aspect of a compression algorithm.
type Compressor interface {
	// Compress returns a reader that yields the decompressed contents of the input reader.
	Compress(in io.ReadCloser) (io.ReadCloser, error)
	algorithm.Algorithm
}

// Decompressor is used to abstract over the decompression aspect of a compression algorithm.
type Decompressor interface {
	// Decompress returns a reader that yields the decompressed contents of the input reader.
	Decompress(in io.Reader) (io.Reader, error)
	algorithm.Algorithm
}
