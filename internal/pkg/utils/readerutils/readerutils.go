package readerutils

import (
	"compress/gzip"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"io"
)

type ReaderChain struct {
	r io.Reader
}

func New(options ...func(*ReaderChain) (io.Reader, error)) (*ReaderChain, error) {
	// TODO: rework this so it fits the actual pattern
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

//func WithGzipCompress(level int, content io.Reader) func(*ReaderChain) (io.Reader, error) {
//	return func(rd *ReaderChain) (io.Reader, error) {
//		return WriterToReader(
//			func(w io.Writer) error {
//				gzr, err := gzip.NewWriterLevel(w, level)
//				if err != nil {
//					return err
//				}
//				defer funcutils.PanicOrLogOnErr(gzr.Close, true, "failed to close gzip writer")
//				_, err = io.Copy(gzr, content)
//				if err != nil {
//					return err
//				}
//				return nil
//			})
//	}
//}

func WithGzipDecompress() func(*ReaderChain) (io.Reader, error) {
	return func(rd *ReaderChain) (io.Reader, error) {
		return gzip.NewReader(rd.r)
	}
}

// EOFCloser Turns io.ReadCloser into io.Reader without leaking by closing the original reader.
func EOFCloser(io.ReadCloser) io.Reader {
	panic("todo")
}

func ChainedCloser(this io.ReadCloser, other io.Closer) io.ReadCloser {
	return struct {
		io.Reader
		io.Closer
	}{
		Reader: this,
		Closer: closerFunc(func() error {
			return funcutils.MultiError(
				other.Close(),
				this.Close(),
			)
		})}
}

// CloserFunc is the basic Close method defined in io.Closer.
type closerFunc func() error

// Close performs close operation by the CloserFunc.
func (fn closerFunc) Close() error {
	return fn()
}

func WriterToReader(reader io.Reader, writerSource func(writer io.Writer) io.Writer) (io.Reader, error) {
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
