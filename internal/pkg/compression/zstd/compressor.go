package zstd

import (
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/unbasical/doras-server/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras-server/pkg/compression"
)

func NewCompressor() compression.Compressor {
	return struct {
		compression.Compressor
	}{
		Compressor: &compressionutils.Compressor{
			Func: func(reader io.ReadCloser) (io.ReadCloser, error) {
				newReader, err := zstd.NewReader(reader)
				if err != nil {
					return nil, err
				}
				return io.NopCloser(newReader), nil
			},
			Algo: "zstd",
		},
	}
}
