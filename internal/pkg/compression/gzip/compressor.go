package gzip

import (
	"compress/gzip"
	"io"

	"github.com/unbasical/doras-server/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras-server/pkg/compression"
)

func NewCompressor() compression.Compressor {
	return struct {
		compression.Compressor
	}{
		Compressor: compressionutils.CompressorFunc(func(reader io.Reader) (io.Reader, error) {
			newReader, err := gzip.NewReader(reader)
			if err != nil {
				return nil, err
			}
			return newReader, nil
		}),
	}
}
