package delta

import (
	"bytes"
	"context"
	"io"
	"path"
	"reflect"
	"testing"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

func TestCreateDelta(t *testing.T) {
	type args struct {
		ctx       context.Context
		src       oras.ReadOnlyTarget
		dst       oras.Target
		fromImage v1.Descriptor
		toImage   v1.Descriptor
		alg       string
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.Descriptor
		wantErr bool
	}{
		// TODO: Add test cases.
		// - tar diff
		// - bsdiff
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateDelta(tt.args.ctx, tt.args.src, tt.args.dst, tt.args.fromImage, tt.args.toImage, tt.args.alg)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDelta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateDelta() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_deltaTag(t *testing.T) {
	type args struct {
		from v1.Descriptor
		to   v1.Descriptor
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "success case", args: struct {
			from v1.Descriptor
			to   v1.Descriptor
		}{from: v1.Descriptor{
			Digest: digest.NewDigestFromHex("sha256", "da39a3ee5e6b4b0d3255bfef95601890afd80709"),
		}, to: v1.Descriptor{
			Digest: digest.NewDigestFromHex("sha256", "da39a3ee5e6b4b0d3255bfef95601890afd80709"),
		}}, want: "sha256-da39a3ee5e6b4b0d3255bfef95601890afd80709_sha256-da39a3ee5e6b4b0d3255bfef95601890afd80709"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deltaTag(tt.args.from, tt.args.to); got != tt.want {
				t.Errorf("deltaTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getBlobReaderForArtifact(t *testing.T) {
	type args struct {
		ctx    context.Context
		src    oras.ReadOnlyTarget
		target v1.Descriptor
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.Descriptor
		want1   io.ReadCloser
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := getBlobReaderForArtifact(tt.args.ctx, tt.args.src, tt.args.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("getBlobReaderForArtifact() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getBlobReaderForArtifact() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("getBlobReaderForArtifact() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_referenceFromDescriptor(t *testing.T) {
	type args struct {
		d v1.Descriptor
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "sucess", args: struct{ d v1.Descriptor }{d: v1.Descriptor{
			Digest: digest.NewDigestFromHex("sha256", "da39a3ee5e6b4b0d3255bfef95601890afd80709"),
		}}, want: "@sha256:da39a3ee5e6b4b0d3255bfef95601890afd80709"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := referenceFromDescriptor(tt.args.d); got != tt.want {
				t.Errorf("referenceFromDescriptor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_writeBlobToTempfile(t *testing.T) {
	tempDir := t.TempDir()
	type args struct {
		outdir string
		target *v1.Descriptor
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "success",
			args: struct {
				outdir string
				target *v1.Descriptor
			}{outdir: tempDir, target: &v1.Descriptor{
				Digest: digest.NewDigestFromHex("sha256", "da39a3ee5e6b4b0d3255bfef95601890afd80709"),
				Size:   int64(len("Hello World!")),
			}}, want: "Hello World!", wantErr: false},
		{name: "not enough bytes",
			args: struct {
				outdir string
				target *v1.Descriptor
			}{outdir: tempDir, target: &v1.Descriptor{
				Digest: digest.NewDigestFromHex("sha256", "da39a3ee5e6b4b0d3255bfef95601890afd80709"),
				Size:   int64(len("Hello World!")) - 1,
			}}, want: "Hello World!", wantErr: true},
		{name: "too many bytes",
			args: struct {
				outdir string
				target *v1.Descriptor
			}{outdir: tempDir, target: &v1.Descriptor{
				Digest: digest.NewDigestFromHex("sha256", "da39a3ee5e6b4b0d3255bfef95601890afd80709"),
				Size:   int64(len("Hello World!")) + 1,
			}}, want: "Hello World!", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantBytes := []byte(tt.want)
			content := bytes.NewReader(wantBytes)
			got, err := writeBlobToTempfile(tt.args.outdir, tt.args.target, content)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeBlobToTempfile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) == tt.wantErr {
				return
			}
			// is the output path correct?
			if got.Name() != path.Join(tempDir, tt.args.target.Digest.Encoded()) {
				t.Errorf("got wrong file path, got=%q want=%q", got.Name(), path.Join(tempDir, tt.args.target.Digest.Encoded()))
			}
			// is the file sought to the beginning?
			if offset, err := got.Seek(0, io.SeekCurrent); err != nil || offset != 0 {
				t.Errorf("seek offset = %v, want = 0, err %v", offset, err)
			}
			data, err := io.ReadAll(got)
			if err != nil {
				t.Errorf("failed to read output file %v", err)
			}
			// does the file content match the input?
			if !bytes.Equal(data, wantBytes) {
				t.Errorf("writeBlobToTempfile() got = %q, want %q", string(data), tt.want)
			}
		})
	}
}
