package readseekcloserwrapper

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"
)

func TestFileBackedReadSeekCloser_Close(t *testing.T) {

	tests := []struct {
		name    string
		r       io.Reader
		wantErr bool
	}{
		{
			name:    "default",
			r:       bytes.NewReader(nil),
			wantErr: false,
		},
		{
			name:    "nil reader",
			r:       nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frsc, err := newImpl(tt.r)
			if err != nil {
				t.Errorf("NewFileBackedReadSeekCloser() error = %v, wantErr %v", err, tt.wantErr)
			}
			if _, err := os.Stat(frsc.filePath); err != nil && errors.Is(err, fs.ErrNotExist) {
				t.Errorf("expected %q to exist: %v", frsc.filePath, err)
			}
			if err := frsc.Close(); (err != nil) != tt.wantErr {
				t.Errorf("Close() error = %v, wantErr %v", err, tt.wantErr)
			}
			if _, err := os.Stat(frsc.filePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("expected %q to exist: %v", frsc.filePath, err)
			}
		})
	}
}

func TestFileBackedReadSeekCloser_Read(t *testing.T) {

	tests := []struct {
		name    string
		want    []byte
		wantErr bool
	}{
		{name: "nil reader", want: nil, wantErr: false},
		{name: "zero length", want: []byte{}, wantErr: false},
		{name: "one read (smaller)", want: []byte("01"), wantErr: false},
		{name: "one read (exact)", want: []byte("0123"), wantErr: false},
		{name: "two read calls", want: []byte("01234567"), wantErr: false},
		{name: "three read calls", want: []byte("012345678"), wantErr: false},
		{name: "many read calls", want: make([]byte, 0xffffff), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedLen := len(tt.want)
			frsc, err := newImpl(bytes.NewReader(tt.want))
			if err != nil {
				t.Errorf("NewFileBackedReadSeekCloser() error = %v, wantErr %v", err, tt.wantErr)
			}
			var got []byte
			for {
				buf := make([]byte, 4)
				n, err := frsc.Read(buf)

				if (err != nil) != tt.wantErr && !errors.Is(err, io.EOF) {
					t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				got = append(got, buf[:n]...)
				if errors.Is(err, io.EOF) {
					break
				}
			}
			if len(got) != expectedLen {
				t.Error("did not read the expected amount of bytes")
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Read() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileBackedReadSeekCloser_Seek_read_then_seek(t *testing.T) {
	data := []byte("012345678")
	rsc, err := newImpl(bytes.NewReader(data))
	if err != nil {
		t.Errorf("NewFileBackedReadSeekCloser() error = %v", err)
	}
	buf := make([]byte, 4)
	_, err = rsc.Read(buf)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	n, err := rsc.Seek(0, io.SeekStart)
	if err != nil {
		t.Errorf("Seek() error = %v", err)
	}
	if n != 0 {
		t.Errorf("Seek() got n = %v, want 0", n)
	}
	got, err := io.ReadAll(rsc)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("Read() got = %v, want %v", got, data)
	}
}

func TestFileBackedReadSeekCloser_Read_then_Seek(t *testing.T) {
	data := []byte("012345678")
	rsc, err := newImpl(bytes.NewReader(data))
	if err != nil {
		t.Errorf("NewFileBackedReadSeekCloser() error = %v", err)
	}
	n, err := rsc.Seek(4, io.SeekStart)
	if err != nil {
		t.Errorf("Seek() error = %v", err)
	}
	if n != 4 {
		t.Errorf("Seek() got n = %v, want 4", n)
	}
	got, err := io.ReadAll(rsc)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	if !bytes.Equal(got, data[4:]) {
		t.Errorf("Read() got = %v, want %v", got, data[4:])
	}
	n, err = rsc.Seek(0, io.SeekStart)
	if err != nil {
		t.Errorf("Seek() error = %v", err)
	}
	if n != 0 {
		t.Errorf("Seek() got n = %v, want 0", n)
	}
	got, err = io.ReadAll(rsc)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("Read() got = %v, want %v", got, data)
	}
}
