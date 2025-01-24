package gzip

import (
	"io"

	"github.com/klauspost/compress/gzip"

	"github.com/unbasical/doras/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras/pkg/algorithm/compression"
)

// NewDecompressor returns a gzip compression.Decompressor.
func NewDecompressor() compression.Decompressor {
	return struct {
		compression.Decompressor
	}{
		Decompressor: &compressionutils.Decompressor{
			Func: func(reader io.Reader) (io.Reader, error) {
				newReader, err := gzip.NewReader(reader)
				if err != nil {
					return nil, err
				}
				return newReader, nil
			},
			Algo: "gzip",
		},
	}
}
