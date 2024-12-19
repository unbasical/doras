package gzip

import (
	"io"

	"github.com/klauspost/compress/gzip"

	"github.com/unbasical/doras-server/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras-server/pkg/compression"
)

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
