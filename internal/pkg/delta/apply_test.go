package delta

import (
	"bytes"
	"compress/gzip"
	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/utils"
	"io"
	"reflect"
	"testing"
)

func TestApplyDelta(t *testing.T) {
	type args struct {
		target  v1.Descriptor
		diff    io.Reader
		content io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    io.ReadCloser
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyDelta(tt.args.target, tt.args.diff, tt.args.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyDelta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ApplyDelta() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBspatch(t *testing.T) {
	from := []byte("Hello")
	to := []byte("Hello World")
	bsDiffPatch, err := bsdiff.Bytes(from, to)

	if err != nil {
		t.Error(err)
	}
	type args struct {
		old   io.Reader
		patch io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{name: "success", args: args{
			old:   bytes.NewReader(from),
			patch: bytes.NewReader(bsDiffPatch),
		}, want: to, wantErr: false},
		{name: "error", args: args{
			old:   bytes.NewReader(from),
			patch: bytes.NewReader([]byte{}),
		}, want: to, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Bspatch(tt.args.old, tt.args.patch)
			data, err := io.ReadAll(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bspatch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (err != nil) && tt.wantErr {
				return
			}

			if !bytes.Equal(data, tt.want) {
				t.Errorf("Bspatch()\ngot = %v,\n want %v", data, tt.want)
			}
		})
	}
}

func TestTarpatch(t *testing.T) {
	diff := utils.ReadOrPanic("test-files/delta.patch.tardiff")
	from := utils.ReadOrPanic("test-files/from.tar.gz")
	to := utils.ReadOrPanic("test-files/to.tar.gz")

	type args struct {
		old   io.Reader
		patch io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{name: "success", args: args{
			old:   bytes.NewReader(from),
			patch: bytes.NewReader(diff),
		}, want: to, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Tarpatch(tt.args.old, tt.args.patch)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bspatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && tt.wantErr {
				return
			}
			data, err := io.ReadAll(got)
			if err != nil {
				t.Error(err)
			}
			gzr, err := gzip.NewReader(bytes.NewReader(tt.want))
			if err != nil {
				t.Error(err)
			}
			if err != nil {
				t.Error(err)
			}
			want, err := io.ReadAll(gzr)
			if err != nil {
				t.Error(err)
			}

			if !bytes.Equal(data, want) {
				t.Errorf("Bspatch()\ngot = %v,\n want %v", data, want)
			}
		})
	}
}
