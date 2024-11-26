package delta

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/unbasical/doras-server/internal/pkg/fileutils"
	"github.com/unbasical/doras-server/internal/pkg/funcutils"

	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	"github.com/unbasical/doras-server/internal/pkg/testutils"
	"oras.land/oras-go/v2/content/oci"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

func TestCreateDelta(t *testing.T) {

	ctx := context.Background()
	dataBinV1 := []byte("Hello")
	dataBinV2 := []byte("Hello World")
	bsdiffData, err := bsdiff.Bytes(dataBinV1, dataBinV2)
	if err != nil {
		t.Fatal(err)
	}
	tardiffData := fileutils.ReadOrPanic("test-files/delta.patch.tardiff")
	dataTarV1 := fileutils.ReadOrPanic("test-files/from.tar.gz")
	dataTarV2 := fileutils.ReadOrPanic("test-files/to.tar.gz")
	src, err := testutils.StorageFromFiles(
		ctx,
		t.TempDir(),
		[]testutils.FileDescription{
			{
				Name: "hello",
				Data: dataBinV1,
				Tag:  "bin-v1",
			},
			{
				Name: "hello",
				Data: dataBinV2,
				Tag:  "bin-v2",
			},
			{
				Name:        "from.tar.gz",
				Data:        dataTarV1,
				Tag:         "tar-v1",
				NeedsUnpack: true,
			},
			{
				Name:        "to.tar.gz",
				Data:        dataTarV2,
				Tag:         "tar-v2",
				NeedsUnpack: true,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	from, err := src.Resolve(ctx, "bin-v1")
	if err != nil {
		t.Fatal(err)
	}
	to, err := src.Resolve(ctx, "bin-v2")
	if err != nil {
		t.Fatal(err)
	}
	fromTar, err := src.Resolve(ctx, "tar-v1")
	if err != nil {
		t.Fatal(err)
	}
	toTar, err := src.Resolve(ctx, "tar-v2")
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
	}
	tests := []struct {
		name        string
		args        args
		wantTag     string
		wantDiff    []byte
		wantErr     bool
		wantDiffAlg string
	}{
		{
			name:        "create delta binary",
			args:        args{ctx: ctx, src: src, dst: dst, fromImage: from, toImage: to},
			wantTag:     deltaTag(from, to),
			wantDiff:    bsdiffData,
			wantErr:     false,
			wantDiffAlg: "bsdiff",
		},
		{
			name:        "create delta tar",
			args:        args{ctx: ctx, src: src, dst: dst, fromImage: fromTar, toImage: toTar},
			wantTag:     deltaTag(fromTar, toTar),
			wantDiff:    tardiffData,
			wantErr:     false,
			wantDiffAlg: "tardiff",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateDelta(tt.args.ctx, tt.args.src, tt.args.dst, tt.args.fromImage, tt.args.toImage)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDelta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			want, err := dst.Resolve(tt.args.ctx, tt.wantTag)
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(*got, want) {
				t.Errorf("CreateDelta() got = %v, want %v", got, tt.wantTag)
			}
			r, err := dst.Fetch(tt.args.ctx, want)
			if err != nil {
				t.Error(err)
			}
			defer funcutils.PanicOrLogOnErr(r.Close, false, "failed to close reader from fetch")
			data, err := io.ReadAll(r)
			if err != nil {
				t.Error(err)
			}
			var manifest v1.Manifest
			err = json.Unmarshal(data, &manifest)
			if err != nil {
				t.Error(err)
			}
			layerDesc := manifest.Layers[0]
			expectedFileName := fmt.Sprintf("%s.patch.%s", deltaTag(tt.args.fromImage, tt.args.toImage), tt.wantDiffAlg)
			gotFileName := layerDesc.Annotations["org.opencontainers.image.title"]
			if gotFileName != expectedFileName {
				t.Errorf("unexpected file name \ngot:%q\nwant%q", gotFileName, expectedFileName)
			}
			dataDiff, err := dst.Fetch(tt.args.ctx, layerDesc)
			if err != nil {
				t.Error(err)
			}
			defer funcutils.PanicOrLogOnErr(dataDiff.Close, false, "failed to close reader from fetch")
			data, err = io.ReadAll(dataDiff)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(data, tt.wantDiff) {
				t.Errorf("CreateDelta()\ngot = %v,\nwant %v", data, tt.wantDiff)
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
		}}, want: "sha256-da39a3ee5e6b4b0d3255bfef9_sha256-da39a3ee5e6b4b0d3255bfef9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deltaTag(tt.args.from, tt.args.to); got != tt.want {
				t.Errorf("deltaTag() = %v, want %v", got, tt.want)
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
			if (err != nil) && tt.wantErr {
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
			expectedExtension: "bsdiff",
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
			want := tt.expectedExtension
			got, got1, err := createDelta(tt.args.fromImage, tt.args.toImage, tt.args.fromReader, tt.args.toReader)
			if (err != nil) != tt.wantErr {
				t.Errorf("createDelta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && tt.wantErr {
				return
			}
			defer funcutils.PanicOrLogOnErr(got1.Close, false, "")
			if got != want {
				t.Errorf("createDelta()\ngot = %v\nwant %v", got, want)
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
			expectedExtension: "tardiff",
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
			want := tt.expectedExtension
			got, got1, err := createDelta(tt.args.fromImage, tt.args.toImage, tt.args.fromReader, tt.args.toReader)
			if (err != nil) != tt.wantErr {
				t.Errorf("createDelta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && tt.wantErr {
				return
			}
			defer funcutils.PanicOrLogOnErr(got1.Close, false, "")
			if got != want {
				t.Errorf("createDelta()\ngot = %v\nwant %v", got, want)
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
