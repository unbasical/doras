package delta

import (
	"bytes"
	"compress/gzip"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/fileutils"
	"io"
	"testing"

	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
)

func TestApplyDelta_Bspatch(t *testing.T) {
	from := []byte("Hello")
	to := []byte("Hello World")
	bsDiffPatch, err := bsdiff.Bytes(from, to)
	if err != nil {
		t.Error(err)
	}
	rc, err := ApplyDelta(
		v1.Descriptor{
			Annotations: map[string]string{
				"org.opencontainers.image.title": "delta.patch.bsdiff",
			},
		},
		bytes.NewReader(bsDiffPatch),
		bytes.NewReader(from),
	)
	if err != nil {
		t.Error(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(to, data) {
		t.Errorf("got %q, want %q", data, to)
	}
}

func TestApplyDelta_Tarpatch(t *testing.T) {
	diff := fileutils.ReadOrPanic("test-files/delta.patch.tardiff")
	from := fileutils.ReadOrPanic("test-files/from.tar.gz")
	to := fileutils.ReadOrPanic("test-files/to.tar.gz")

	rc, err := ApplyDelta(
		v1.Descriptor{
			Annotations: map[string]string{
				"org.opencontainers.image.title": "delta.patch.tardiff",
			},
		},
		bytes.NewReader(diff),
		bytes.NewReader(from),
	)
	if err != nil {
		t.Error(err)
	}
	defer rc.Close()
	gz, err := gzip.NewReader(bytes.NewReader(to))
	if err != nil {
		t.Error(err)
	}
	// the output is not compressed, so we compare it to the decompressed version of `to`
	want, err := io.ReadAll(gz)
	if err != nil {
		t.Error(err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("got %q, want %q", got, want)
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
			got, _ := Bspatch(tt.args.old, tt.args.patch)
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
	diff := fileutils.ReadOrPanic("test-files/delta.patch.tardiff")
	from := fileutils.ReadOrPanic("test-files/from.tar.gz")
	to := fileutils.ReadOrPanic("test-files/to.tar.gz")

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
