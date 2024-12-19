package gzip

import (
	"bytes"
	"io"
	"testing"

	"github.com/klauspost/compress/gzip"
)

func TestCompressor(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{name: "Empty", input: make([]byte, 0)},
		{name: "Non empty", input: []byte("foo")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressor := NewCompressor()
			input := io.NopCloser(bytes.NewReader(tt.input))
			rc, err := compressor.Compress(input)
			if err != nil {
				t.Error(err)
				return
			}
			got, err := io.ReadAll(rc)
			if err != nil {
				t.Error(err)
				return
			}
			buf := bytes.NewBuffer(make([]byte, 0))
			w := gzip.NewWriter(buf)
			if err != nil {
				t.Fatal(err)
			}
			_, err = io.Copy(w, bytes.NewBuffer(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			_ = w.Close()
			want := buf.Bytes()
			if !bytes.Equal(got, want) {
				t.Errorf("got %v, want %v", got, want)
				return
			}
		})
	}
}

func TestNewDecompressor(t *testing.T) {
	tests := []struct {
		name string
		want []byte
	}{
		{name: "Empty", want: make([]byte, 0)},
		{name: "Non empty", want: []byte("foo")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer(make([]byte, 0))
			gzw := gzip.NewWriter(buf)
			_, err := gzw.Write(tt.want)
			if err != nil {
				t.Fatal(err)
			}
			_ = gzw.Close()
			compressed := buf.Bytes()
			decompressor := NewDecompressor()
			r, err := decompressor.Decompress(bytes.NewReader(compressed))
			if err != nil {
				t.Error(err)
				return
			}
			got, err := io.ReadAll(r)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
