package zstd

import (
	"github.com/klauspost/compress/zstd"
	"github.com/unbasical/doras/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras/internal/pkg/utils/readerutils"
	"github.com/unbasical/doras/pkg/algorithm/compression"
	"io"
)

// NewCompressor returns a zstd compression.Compressor.
func NewCompressor() compression.Compressor {
	return struct {
		compression.Compressor
	}{
		// Turn the compression writer into a reader.
		Compressor: &compressionutils.Compressor{
			Func: func(reader io.ReadCloser) (io.ReadCloser, error) {
				var closer readerutils.CloserFunc
				r := readerutils.WriterToReader(reader, func(writer io.Writer) io.WriteCloser {
					newWriter, err := zstd.NewWriter(writer)
					if err != nil {
						panic(err)
					}
					closer = newWriter.Close
					return newWriter
				})
				retval := struct {
					io.Reader
					io.Closer
				}{
					Reader: r,
					Closer: closer,
				}
				// Prevent resource leak.
				return readerutils.ChainedCloser(retval, reader), nil
			},
			Algo: "zstd",
		},
	}
}
