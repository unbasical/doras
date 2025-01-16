package tarfsdatasource

import (
	"archive/tar"
	"errors"
	"fmt"
	tarpatch "github.com/containers/tar-diff/pkg/tar-patch"
	"github.com/unbasical/doras-server/internal/pkg/utils/readseekcloserwrapper"
	"io"
	"path"
)

// entry represents an entry in a tar archive.
type entry struct {
	header *tar.Header
	pos    int64
}

// DataSource implements a file system backed tarpatch.DataSource.
// This does not extract the archive, but it creates a temporary file, to which the contents are written lazily.
type DataSource struct {
	rsc          io.ReadSeekCloser
	entries      map[string]*entry
	currentEntry *entry
	currentPos   int64
}

// NewDataSource constructs a tarpatch.DataSource that optionally decompresses the input archive and writes the archive to a temporary file.
// Calling Close() cleans up the temporary files.
func NewDataSource(r io.Reader, decompress func(reader io.Reader) (io.Reader, error)) (tarpatch.DataSource, error) {
	if decompress != nil {
		var err error
		r, err = decompress(r)
		if err != nil {
			return nil, err
		}
	}
	res := &DataSource{
		entries: make(map[string]*entry),
	}
	rsc, err := readseekcloserwrapper.New(r)
	if err != nil {
		return nil, err
	}
	res.rsc = rsc
	tr := tar.NewReader(rsc)
	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		pos, err := rsc.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		res.entries[path.Clean(header.Name)] = &entry{
			header: header,
			pos:    pos,
		}
	}
	return res, nil
}

func (t *DataSource) Read(p []byte) (n int, err error) {
	if t.currentEntry == nil {
		return 0, fmt.Errorf("no file set")
	}
	bytesLeftInFile := t.currentEntry.header.Size - (t.currentPos - t.currentEntry.pos)

	n, err = io.ReadFull(io.LimitReader(t.rsc, bytesLeftInFile), p)
	t.currentPos += int64(n)
	if err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return n, io.EOF
		}
		return n, err
	}
	return n, nil
}

func (t *DataSource) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = t.currentEntry.pos + offset
	case io.SeekCurrent:
		newPos = t.currentPos + offset
	case io.SeekEnd:
		newPos = t.currentEntry.header.Size + t.currentEntry.pos + offset
	}
	if newPos < t.currentEntry.pos || newPos > t.currentEntry.header.Size+t.currentEntry.pos {
		return 0, fmt.Errorf("seek out of range")
	}
	// always use SeekStart because we already calculated the offset
	offset, err := t.rsc.Seek(newPos, io.SeekStart)
	if err != nil {
		return offset, err
	}
	t.currentPos = offset
	return offset - t.currentEntry.pos, nil
}

func (t *DataSource) Close() error {
	// do nothing as close means closing the current file.
	return nil
}

// CloseDataSource Close the currently opened reader.
func (t *DataSource) CloseDataSource() error {
	return t.rsc.Close()
}

func (t *DataSource) SetCurrentFile(file string) error {
	e, ok := t.entries[file]
	if !ok {
		return fmt.Errorf("file not found")
	}
	t.currentEntry = e
	t.currentPos = e.pos
	// this calls the current object, it will seek accordingly because we updated the internal state correctly
	_, err := t.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	return nil
}
