package readerutils

import (
	"bytes"
	"github.com/klauspost/compress/gzip"
	"io"
	"testing"
)

func TestWriterToReader(t *testing.T) {
	type args struct {
		content      io.Reader
		writerSource func(writer io.Writer) io.WriteCloser
	}

	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "pass through writer",
			args: args{
				content: bytes.NewBuffer([]byte{1, 2, 3}),
				writerSource: func(writer io.Writer) io.WriteCloser {
					return struct {
						io.Writer
						io.Closer
					}{
						Writer: writer,
						Closer: closerFunc(func() error { return nil }),
					}
				},
			},
			want:    []byte{1, 2, 3},
			wantErr: false,
		},
		{
			name: "intermediate writer",
			args: args{
				content: bytes.NewBuffer([]byte{1, 2, 3}),
				writerSource: func(writer io.Writer) io.WriteCloser {
					w := gzip.NewWriter(writer)
					return w
				},
			},
			want: func() []byte {
				buf := bytes.NewBuffer(make([]byte, 0))
				w := gzip.NewWriter(buf)
				_, err := w.Write([]byte{1, 2, 3})
				if err != nil {
					t.Fatal(err)
				}
				_ = w.Close()
				return buf.Bytes()
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WriterToReader(tt.args.content, tt.args.writerSource)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriterToReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotBytes, err := io.ReadAll(got)
			if err != nil {
				t.Errorf("ReadAll() error = %v", err)
				return
			}
			if !bytes.Equal(gotBytes, tt.want) {
				t.Errorf("WriterToReader() got = %v, want %v", gotBytes, tt.want)
			}
		})
	}
}
