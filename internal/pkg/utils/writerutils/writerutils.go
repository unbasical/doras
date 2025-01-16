package writerutils

import (
	"errors"
	"io"
	"os"
)

// safeFile wraps around an *os.File and sync file changes with the disk when an EOF is reached or the file is closed.
type safeFile struct {
	f *os.File
}

// NewSafeFileWriter return an io.WriteCloser that wraps around an *os.File.
// It syncs file changes with the disk when an EOF is reached or the file is closed.
// It is based on the Sync() function from os.File.
func NewSafeFileWriter(f *os.File) io.WriteCloser {
	return &safeFile{f: f}
}

func (s *safeFile) Write(p []byte) (n int, err error) {
	n, err = s.f.Write(p)
	if err != nil {
		if errors.Is(err, io.EOF) {
			errSync := s.f.Sync()
			if errSync != nil {
				return n, errSync
			}
		}
	}
	return n, err
}

func (s *safeFile) Close() error {
	return errors.Join(
		s.f.Sync(),
		s.f.Close(),
	)
}
