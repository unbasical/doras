package readseekcloserwrapper

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
)

type FileBackedReadSeekCloser struct {
	r io.Reader
	// this file is automatically appended to when the provided reader is read from
	fileIngest *os.File
	// points to the same file as fileIngest but is used only for reading
	fileOut  *os.File
	flusher  *bufio.Writer
	filePath string
	// stores the offset within the reader r
	offsetReader int64
	// stores the offset within the reading file
	offsetFile int64
}

func newImpl(r io.Reader) (*FileBackedReadSeekCloser, error) {
	res := &FileBackedReadSeekCloser{}
	tmpFile, err := os.CreateTemp(os.TempDir(), "rsc_*")
	if err != nil {
		return nil, fmt.Errorf("error creating temp file for writing: %w", err)
	}
	res.fileIngest = tmpFile
	res.filePath = res.fileIngest.Name()
	res.flusher = bufio.NewWriter(res.fileIngest)
	res.r = io.TeeReader(r, res.flusher)
	fOut, err := os.OpenFile(res.filePath, os.O_RDONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("error opening temp file for reading: %w", err)
	}
	res.fileOut = fOut
	return res, nil
}

func New(r io.Reader) (io.ReadSeekCloser, error) {
	return newImpl(r)
}

// Read reads from the original ReadCloser or the temporary file if it has been created.
func (frsc *FileBackedReadSeekCloser) Read(p []byte) (int, error) {
	if frsc.offsetReader < frsc.offsetFile {
		bytesBehind := frsc.offsetFile - frsc.offsetReader
		data, err := io.ReadAll(io.LimitReader(frsc.r, bytesBehind))
		if err != nil {
			return 0, err
		}
		frsc.offsetReader += int64(len(data))
	}
	// this means we can read from the provided reader
	if frsc.offsetReader == frsc.offsetFile {
		n, err := frsc.r.Read(p)
		if err != nil {
			return n, err
		}
		frsc.offsetReader += int64(n)
		frsc.offsetFile += int64(n)
		return n, nil
	}
	// flush output file so we can read from the same file without it being outdated
	err := frsc.flusher.Flush()
	if err != nil {
		return 0, err
	}
	// in this chunk of code we assume we have to read from the file first and then from the reader
	bytesFromFile := max(frsc.offsetReader-frsc.offsetFile, 0)
	_, err = frsc.fileOut.Seek(frsc.offsetFile, io.SeekStart)
	if err != nil {
		return 0, err
	}
	tmpReader := io.MultiReader(io.LimitReader(frsc.fileOut, bytesFromFile), frsc.r)
	n, err := io.ReadFull(tmpReader, p)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return n, err
	}

	frsc.offsetReader += max(int64(n)-bytesFromFile, 0)
	frsc.offsetFile += int64(n)
	return n, nil
}

func (frsc *FileBackedReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	_, err := frsc.fileOut.Seek(frsc.offsetFile, io.SeekStart)
	if err != nil {
		return 0, err
	}
	offset, err = frsc.fileOut.Seek(offset, whence)
	frsc.offsetFile = offset
	return offset, err
}

func (frsc *FileBackedReadSeekCloser) Close() error {
	closer := func(f *os.File) {
		if err := f.Close(); err != nil {
			fmt.Printf("error closing file: %v\n", err)
		}
	}
	defer closer(frsc.fileOut)
	defer closer(frsc.fileIngest)
	err := os.Remove(frsc.filePath)
	if err != nil {
		return err
	}
	return nil
}
