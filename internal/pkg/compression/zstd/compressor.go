package zstd

import (
	"io"

	"github.com/unbasical/doras-server/internal/pkg/utils/readerutils"

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
				r := readerutils.WriterToReader(reader, func(writer io.Writer) io.WriteCloser {
					newWriter, err := zstd.NewWriter(writer)
					if err != nil {
						panic(err)
					}
					return newWriter
				})
				return io.NopCloser(r), nil
			},
			Algo: "zstd",
		},
	}
}
