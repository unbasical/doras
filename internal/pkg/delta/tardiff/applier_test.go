package tardiff

import (
	"bytes"
	"compress/gzip"
	"github.com/opencontainers/go-digest"
	gzip2 "github.com/unbasical/doras/internal/pkg/compression/gzip"
	"github.com/unbasical/doras/internal/pkg/utils/tarutils"
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"io"
	"os"
	"testing"

	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
)

func TestApplier_Apply(t *testing.T) {
	applier := &applier{}

	diff := fileutils.ReadOrPanic("../../../../test/test-files/delta.patch.tardiff")
	from := fileutils.ReadOrPanic("../../../../test/test-files/from.tar.gz")
	to := fileutils.ReadOrPanic("../../../../test/test-files/to.tar.gz")

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
			got, err := applier.Patch(tt.args.old, tt.args.patch)
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

func Test_patcher_PatchFilesystem(t *testing.T) {
	type args struct {
		patch    io.Reader
		expected *digest.Digest
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success (without digest)", args: args{
				patch:    bytes.NewReader(fileutils.ReadOrPanic("../../../../test/test-files/delta.patch.tardiff")),
				expected: nil,
			},
			wantErr: false,
		},
		{
			name: "success (with digest)", args: args{
				patch: bytes.NewReader(fileutils.ReadOrPanic("../../../../test/test-files/delta.patch.tardiff")),
				expected: func() *digest.Digest {
					data := fileutils.ReadOrPanic("../../../../test/test-files/to.tar.gz")
					r, err := gzip2.NewDecompressor().Decompress(bytes.NewReader(data))
					if err != nil {
						panic(err)
					}
					d, err := digest.FromReader(r)
					if err != nil {
						panic(err)
					}
					return &d
				}(),
			},
			wantErr: false,
		},
		{
			name: "failure (bad patch)", args: args{
				patch:    bytes.NewReader(nil),
				expected: nil,
			},
			wantErr: true,
		},
		{
			name: "failure (bad digest)", args: args{
				patch:    bytes.NewReader(fileutils.ReadOrPanic("../../../../test/test-files/delta.patch.tardiff")),
				expected: func() *digest.Digest { d := digest.FromBytes(nil); return &d }(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outDir, err := os.MkdirTemp(t.TempDir(), "output-dir-*")
			if err != nil {
				t.Fatal(err)
			}
			expectedDir, err := os.MkdirTemp(t.TempDir(), "expected-dir-*")
			if err != nil {
				t.Fatal(err)
			}
			expectedDirTarPath := func() string {
				if tt.wantErr {
					return "../../../../test/test-files/from.tar.gz"
				}
				return "../../../../test/test-files/to.tar.gz"
			}()
			err = tarutils.ExtractCompressedTar(expectedDir, "", expectedDirTarPath, nil, gzip2.NewDecompressor())
			if err != nil {
				t.Fatal(err)
				return
			}

			err = tarutils.ExtractCompressedTar(outDir, "", "../../../../test/test-files/from.tar.gz", nil, gzip2.NewDecompressor())
			if err != nil {
				t.Fatal(err)
				return
			}
			a := &applier{}
			err = a.PatchFilesystem(outDir, tt.args.patch, tt.args.expected)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PatchFilesystem() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			eq, cmpErr := fileutils.CompareDirectories(outDir, expectedDir)
			if !eq {
				t.Fatalf("output directory does not match expected directory: %v", cmpErr)
			}
		})
	}
}

func TestPatcher_Interface(t *testing.T) {
	var c any = &applier{}
	_, ok := (c).(delta.Patcher)
	if !ok {
		t.Error("interface not implemented")
	}
}
