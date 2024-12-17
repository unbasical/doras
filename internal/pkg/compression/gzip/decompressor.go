package gzip

import (
	"compress/gzip"
	"io"

	"github.com/unbasical/doras-server/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras-server/pkg/compression"
)

func NewDecompressor() compression.Decompressor {
	return struct {
		compression.Decompressor
	}{
		Decompressor: &compressionutils.Decompressor{
			Func: func(reader io.Reader) (io.Reader, error) {
				return writerToReader(reader, func(writer io.Writer) io.Writer {
					return gzip.NewWriter(writer)
				})
			},
			Algo: "gzip",
		},
	}
}

func writerToReader(reader io.Reader, writerSource func(writer io.Writer) io.Writer) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		gzr := writerSource(pw)
		_, err := io.ReadAll(io.TeeReader(reader, gzr))
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()
	return pr, nil
}
