package readerutils

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
)

func ChainedCloser(this io.ReadCloser, other io.Closer) io.ReadCloser {
	return struct {
		io.Reader
		io.Closer
	}{
		Reader: this,
		Closer: closerFunc(func() error {
			return errors.Join(
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

// WriterToReader transforms an io.Writer that is provided by the given function into an io.Reader.
func WriterToReader(reader io.Reader, writerSource func(writer io.Writer) io.WriteCloser) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		gzr := writerSource(pw)
		n, err := io.Copy(gzr, reader)
		errClose := gzr.Close()
		if err := errors.Join(err, errClose); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		log.Debugf("wrote %d bytes", n)
		_ = pw.Close()
	}()
	return pr
}
