package readerutils

import (
	"compress/gzip"
	"io"
)

type ReaderChain struct {
	r io.Reader
}

// TODO: rework this so it fits the actual pattern
func New(options ...func(*ReaderChain) (io.Reader, error)) (*ReaderChain, error) {
	rd := &ReaderChain{}
	for _, o := range options {
		r, err := o(rd)
		if err != nil {
			return nil, err
		}
		rd.r = r
	}
	return rd, nil
}

func WithGzipCompress(level int, content io.Reader) func(*ReaderChain) (io.Reader, error) {
	return func(rd *ReaderChain) (io.Reader, error) {
		return WriterToReader(
			func(w io.Writer) error {
				gzr, err := gzip.NewWriterLevel(w, level)
				if err != nil {
					return err
				}
				defer gzr.Close()
				_, err = io.Copy(gzr, content)
				if err != nil {
					return err
				}
				return nil
			})
	}
}

func WithGzipDecompress() func(*ReaderChain) (io.Reader, error) {
	return func(rd *ReaderChain) (io.Reader, error) {
		return gzip.NewReader(rd.r)
	}
}

func WriterToReader(f func(io.Writer) error) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		err := f(pw)
		if err != nil {
			_ = pw.CloseWithError(err)
		}

		_ = pw.Close()
	}()
	return pr, nil
}

// EOFCloser Turns io.ReadCloser into io.Reader without leaking by closing the original reader.
func EOFCloser(io.ReadCloser) io.Reader {
	panic("todo")
}
