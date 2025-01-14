package readerutils

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"io"
)

type ReaderChain struct {
	r io.Reader
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
