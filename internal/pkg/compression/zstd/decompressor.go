package zstd

import (
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/unbasical/doras-server/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras-server/internal/pkg/utils/readerutils"
	"github.com/unbasical/doras-server/pkg/compression"
)

func NewDecompressor() compression.Decompressor {
	return struct {
		compression.Decompressor
	}{
		Decompressor: &compressionutils.Decompressor{
			Func: func(reader io.Reader) (io.Reader, error) {
				return readerutils.WriterToReader(reader, func(writer io.Writer) io.Writer {
					newWriter, err := zstd.NewWriter(writer)
					if err != nil {
						panic(err)
					}
					return newWriter
				})
			},
			Algo: "zstd",
		},
	}
}
