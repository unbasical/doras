package delta

import (
	"bytes"
	"context"
	"crypto/sha256"
	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	"github.com/unbasical/doras-server/internal/pkg/testutils"
	"io"
	"oras.land/oras-go/v2/content/oci"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

func TestCreateDelta(t *testing.T) {
	// TODO: finish this test
	src, err := testutils.StorageFromFiles(
		context.Background(),
		t.TempDir(),
		map[string]testutils.FileDescription{
			"hello": {
				Data: strings.NewReader("Hello"),
			},
		},
		"test",
	)
	if err != nil {
		t.Fatal(err)
	}
	dst, err := oci.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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
		want    v1.Descriptor
		wantErr bool
	}{
		{name: "create delta", args: struct {
			ctx       context.Context
			src       oras.ReadOnlyTarget
			dst       oras.Target
			fromImage v1.Descriptor
			toImage   v1.Descriptor
			alg       string
		}{ctx: context.Background(), src: src, dst: dst, fromImage: v1.Descriptor{
			MediaType:    "",
			Digest:       "",
			Size:         0,
			URLs:         nil,
			Annotations:  nil,
			Data:         nil,
			Platform:     nil,
			ArtifactType: "",
		}, toImage: v1.Descriptor{
			MediaType:    "",
			Digest:       "",
			Size:         0,
			URLs:         nil,
			Annotations:  nil,
			Data:         nil,
			Platform:     nil,
			ArtifactType: "",
		}, alg: "bsdiff"}, want: v1.Descriptor{}, wantErr: false},
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

func Test_createDeltaBinary(t *testing.T) {
	from := []byte("hello")
	fromDigest := sha256.Sum256(from)
	to := []byte("hello world")
	toDigest := sha256.Sum256(to)
	patch, err := bsdiff.Bytes(from, to)
	if err != nil {
		panic(err)
	}
	type args struct {
		fromImage  v1.Descriptor
		toImage    v1.Descriptor
		fromReader io.ReadSeeker
		toReader   io.ReadSeeker
	}
	tests := []struct {
		name              string
		args              args
		expectedExtension string
		want1             io.Reader
		wantErr           bool
	}{
		{name: "success", args: struct {
			fromImage  v1.Descriptor
			toImage    v1.Descriptor
			fromReader io.ReadSeeker
			toReader   io.ReadSeeker
		}{fromImage: v1.Descriptor{
			MediaType: "application/vnd.oci.image.layer.v1.tar",
			Digest:    digest.NewDigestFromBytes("sha256", fromDigest[:]),
			Size:      int64(len(from)),
			Annotations: map[string]string{
				"org.opencontainers.image.title": "foo",
			},
			Platform:     nil,
			ArtifactType: "",
		}, toImage: v1.Descriptor{
			MediaType: "application/vnd.oci.image.layer.v1.tar",
			Digest:    digest.NewDigestFromBytes("sha256", toDigest[:]),
			Size:      int64(len(to)),
			Annotations: map[string]string{
				"org.opencontainers.image.title": "foo",
			},
			ArtifactType: "",
		}, fromReader: bytes.NewReader(from), toReader: bytes.NewReader(to)},
			want1:             bytes.NewReader(patch),
			expectedExtension: ".patch.bsdiff",
			wantErr:           false,
		},
		{name: "unpack mismatch error", args: struct {
			fromImage  v1.Descriptor
			toImage    v1.Descriptor
			fromReader io.ReadSeeker
			toReader   io.ReadSeeker
		}{fromImage: v1.Descriptor{
			Annotations: map[string]string{
				ContentUnpack: "true",
			},
		}, toImage: v1.Descriptor{}, fromReader: nil, toReader: nil},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := deltaTag(tt.args.fromImage, tt.args.toImage) + tt.expectedExtension
			got, got1, err := createDelta(tt.args.fromImage, tt.args.toImage, tt.args.fromReader, tt.args.toReader)
			if (err != nil) != tt.wantErr {
				t.Errorf("createDelta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) == tt.wantErr {
				return
			}
			defer got1.Close()
			if *got != want {
				t.Errorf("createDelta()\ngot = %v\nwant %v", *got, want)
			}
			gotBytes, err := io.ReadAll(got1)
			if err != nil {
				t.Error(err)
			}
			wantBytes, err := io.ReadAll(tt.want1)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(gotBytes, wantBytes) {
				t.Errorf("createDelta()\ngot1 = %v\nwant %v", gotBytes, wantBytes)
			}
		})
	}
}

func Test_createDeltaTardiff(t *testing.T) {
	from, err := os.ReadFile("./test-files/from.tar.gz")
	if err != nil {
		t.Error(err)
	}
	to, err := os.ReadFile("./test-files/to.tar.gz")
	if err != nil {
		t.Error(err)
	}
	fromDigest := sha256.Sum256(from)
	toDigest := sha256.Sum256(to)
	patch, err := os.ReadFile("./test-files/delta.patch.tardiff")
	if err != nil {
		t.Error(err)
	}

	type args struct {
		fromImage  v1.Descriptor
		toImage    v1.Descriptor
		fromReader io.ReadSeeker
		toReader   io.ReadSeeker
	}
	tests := []struct {
		name              string
		args              args
		expectedExtension string
		want1             io.Reader
		wantErr           bool
	}{
		{name: "success", args: struct {
			fromImage  v1.Descriptor
			toImage    v1.Descriptor
			fromReader io.ReadSeeker
			toReader   io.ReadSeeker
		}{fromImage: v1.Descriptor{
			MediaType: "application/vnd.oci.image.layer.v1.tar",
			Digest:    digest.NewDigestFromBytes("sha256", fromDigest[:]),
			Size:      int64(len(from)),
			Annotations: map[string]string{
				"org.opencontainers.image.title": "foo",
				ContentUnpack:                    "true",
			},
			Platform:     nil,
			ArtifactType: "",
		}, toImage: v1.Descriptor{
			MediaType: "application/vnd.oci.image.layer.v1.tar",
			Digest:    digest.NewDigestFromBytes("sha256", toDigest[:]),
			Size:      int64(len(to)),
			Annotations: map[string]string{
				"org.opencontainers.image.title": "foo",
				ContentUnpack:                    "true",
			},
			ArtifactType: "",
		}, fromReader: bytes.NewReader(from), toReader: bytes.NewReader(to)},
			want1:             bytes.NewReader(patch),
			expectedExtension: ".patch.tardiff",
			wantErr:           false,
		},
		{name: "unpack mismatch error", args: struct {
			fromImage  v1.Descriptor
			toImage    v1.Descriptor
			fromReader io.ReadSeeker
			toReader   io.ReadSeeker
		}{fromImage: v1.Descriptor{
			Annotations: map[string]string{
				ContentUnpack: "true",
			},
		}, toImage: v1.Descriptor{
			Annotations: map[string]string{
				ContentUnpack: "false",
			},
		}, fromReader: bytes.NewReader(from), toReader: bytes.NewReader(to)},
			want1:   nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := deltaTag(tt.args.fromImage, tt.args.toImage) + tt.expectedExtension
			got, got1, err := createDelta(tt.args.fromImage, tt.args.toImage, tt.args.fromReader, tt.args.toReader)
			if (err != nil) != tt.wantErr {
				t.Errorf("createDelta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) == tt.wantErr {
				return
			}
			defer got1.Close()
			if *got != want {
				t.Errorf("createDelta()\ngot = %v\nwant %v", *got, want)
			}
			gotBytes, err := io.ReadAll(got1)
			if err != nil {
				t.Error(err)
			}
			wantBytes, err := io.ReadAll(tt.want1)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(gotBytes, wantBytes) {
				t.Errorf("createDelta()\ngot1 = %v\nwant %v", gotBytes, wantBytes)
			}
		})
	}
}
