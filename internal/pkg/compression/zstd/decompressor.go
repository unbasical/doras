package zstd

import (
	"io"

	"github.com/klauspost/compress/zstd"

	"github.com/unbasical/doras-server/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras-server/pkg/algorithm/compression"
)

// NewDecompressor returns a zstd compression.Decompressor.
func NewDecompressor() compression.Decompressor {
	return struct {
		compression.Decompressor
	}{
		// Construct the Decompressor with a function that returns a zstd reader.
		Decompressor: &compressionutils.Decompressor{
			Func: func(reader io.Reader) (io.Reader, error) {
				newReader, err := zstd.NewReader(reader)
				if err != nil {
					return nil, err
				}
				return newReader, nil
			},
			Algo: "zstd",
		},
	}
}
