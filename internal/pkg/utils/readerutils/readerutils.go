package readerutils

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

func ChainedCloser(this io.ReadCloser, other io.Closer) io.ReadCloser {
	if this == nil {
		panic("this is nil")
	}
	if other == nil {
		panic("other is nil")
	}
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
	wg := sync.WaitGroup{}
	wg.Add(1)
	pr, pw := io.Pipe()
	go func() {
		gzr := writerSource(pw)
		wg.Done()
		n, err := io.Copy(gzr, reader)
		errClose := gzr.Close()
		if err := errors.Join(err, errClose); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		log.Debugf("wrote %d bytes", n)
		_ = pw.Close()
	}()
	wg.Wait()
	return pr
}

// CloserFunc is the basic Close method defined in io.Closer.
type CloserFunc func() error

// Close performs close operation by the CloserFunc.
func (fn CloserFunc) Close() error {
	return fn()
}

// LatencyReader wraps an io.Reader and introduces a delay before each read.
type LatencyReader struct {
	Reader io.Reader
	Delay  time.Duration
}

func (l *LatencyReader) Read(p []byte) (int, error) {
	time.Sleep(l.Delay) // Introduce latency
	return l.Reader.Read(p)
}

// CleanupReadCloser wraps an io.ReadCloser and a function to be called on Close.
type CleanupReadCloser struct {
	reader      io.ReadCloser
	cleanupFunc func() error
}

// NewCleanupReadCloser creates an io.ReadCloser that calls the cleanup function after closing the original closer.
// This is useful for cleaning up temporary files.
func NewCleanupReadCloser(r io.ReadCloser, cleanup func() error) io.ReadCloser {
	return &CleanupReadCloser{reader: r, cleanupFunc: cleanup}
}

// Read delegates to the underlying reader.
func (f *CleanupReadCloser) Read(p []byte) (int, error) {
	return f.reader.Read(p)
}

// Close closes the reader and calls the provided function.
func (f *CleanupReadCloser) Close() error {
	return errors.Join(
		f.reader.Close(),
		f.cleanupFunc(),
	)
}

// CountingReader is a wrapper around io.ReadCloser that tracks the total number of bytes read using an atomic counter.
type CountingReader struct {
	bytesRead *atomic.Uint64
	rc        io.ReadCloser
}

// NewCountingReader creates a wrapper around io.ReadCloser that tracks the total bytes read using an atomic counter.
func NewCountingReader(rc io.ReadCloser, bytesRead *atomic.Uint64) io.ReadCloser {
	return &CountingReader{
		rc:        rc,
		bytesRead: bytesRead,
	}
}

func (c *CountingReader) Read(p []byte) (n int, err error) {
	n, err = c.rc.Read(p)
	if err != nil {
		return n, err
	}
	c.bytesRead.Add(uint64(n))
	return n, nil
}

func (c *CountingReader) Close() error {
	return c.rc.Close()
}
