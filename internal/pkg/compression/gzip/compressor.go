package gzip

import (
	"io"

	"github.com/unbasical/doras-server/internal/pkg/utils/readerutils"

	"github.com/klauspost/compress/gzip"

	"github.com/unbasical/doras-server/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras-server/pkg/algorithm/compression"
)

// CloserFunc is the basic Close method defined in io.Closer.
type CloserFunc func() error

// Close performs close operation by the CloserFunc.
func (fn CloserFunc) Close() error {
	return fn()
}

// NewCompressor returns a gzip compression.Compressor.
func NewCompressor() compression.Compressor {
	return struct {
		compression.Compressor
	}{
		Compressor: &compressionutils.Compressor{
			Func: func(reader io.ReadCloser) (io.ReadCloser, error) {
				var closer CloserFunc
				r := readerutils.WriterToReader(reader, func(writer io.Writer) io.WriteCloser {
					gzw := gzip.NewWriter(writer)
					closer = gzw.Close
					return gzw
				})
				retval := struct {
					io.Reader
					io.Closer
				}{
					Reader: r,
					Closer: closer,
				}
				return readerutils.ChainedCloser(retval, reader), nil
			},
			Algo: "gzip",
		},
	}
}
