package writerutils

import (
	"errors"
	"io"
	"os"
)

type SafeFile struct {
	f *os.File
}

func NewSafeFileWriter(f *os.File) io.WriteCloser {
	return &SafeFile{f: f}
}

func (s SafeFile) Write(p []byte) (n int, err error) {
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

func (s SafeFile) Close() error {
	return errors.Join(
		s.f.Sync(),
		s.f.Close(),
	)
}
