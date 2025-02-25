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
	applier := NewPatcher()

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
		from            string
		patch           []byte
		expectedDirPath string
		expected        *digest.Digest
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success (without digest, overlapping archives)", args: args{
				from:            "../../../../test/test-files/to.tar.gz",
				patch:           fileutils.ReadOrPanic("../../../../test/test-files/delta.patch.tardiff"),
				expectedDirPath: "../../../../test/test-files/to.tar.gz",
				expected:        nil,
			},
			wantErr: false,
		},
		{
			name: "success (with digest, overlapping archives)", args: args{
				from:            "../../../../test/test-files/to.tar.gz",
				patch:           fileutils.ReadOrPanic("../../../../test/test-files/delta.patch.tardiff"),
				expectedDirPath: "../../../../test/test-files/to.tar.gz",
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
			name: "success (without digest, non-overlapping archives)", args: args{
				from:            "../../../../test/test-files/01-from.tar.gz",
				patch:           fileutils.ReadOrPanic("../../../../test/test-files/01-delta.patch.tardiff"),
				expectedDirPath: "../../../../test/test-files/01-to.tar.gz",
				expected:        nil,
			},
			wantErr: false,
		},
		{
			name: "success (with digest, non-overlapping archives)", args: args{
				from:            "../../../../test/test-files/01-from.tar.gz",
				patch:           fileutils.ReadOrPanic("../../../../test/test-files/01-delta.patch.tardiff"),
				expectedDirPath: "../../../../test/test-files/01-to.tar.gz",
				expected: func() *digest.Digest {
					data := fileutils.ReadOrPanic("../../../../test/test-files/01-to.tar.gz")
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
				from:            "../../../../test/test-files/from.tar.gz",
				patch:           nil,
				expected:        nil,
				expectedDirPath: "../../../../test/test-files/from.tar.gz",
			},
			wantErr: true,
		},
		{
			name: "failure (bad digest)", args: args{
				from:            "../../../../test/test-files/from.tar.gz",
				patch:           fileutils.ReadOrPanic("../../../../test/test-files/delta.patch.tardiff"),
				expected:        func() *digest.Digest { d := digest.FromBytes(nil); return &d }(),
				expectedDirPath: "../../../../test/test-files/from.tar.gz",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, keepOldDir := range []bool{true, false} {
				outDir, err := os.MkdirTemp(t.TempDir(), "output-dir-*")
				if err != nil {
					t.Fatal(err)
				}
				expectedDir, err := os.MkdirTemp(t.TempDir(), "expected-dir-*")
				if err != nil {
					t.Fatal(err)
				}
				expectedDirTarPath := tt.args.expectedDirPath
				err = tarutils.ExtractCompressedTar(expectedDir, "", expectedDirTarPath, nil, gzip2.NewDecompressor())
				if err != nil {
					t.Fatal(err)
					return
				}

				err = tarutils.ExtractCompressedTar(outDir, "", tt.args.from, nil, gzip2.NewDecompressor())
				if err != nil {
					t.Fatal(err)
					return
				}
				patcherDir := t.TempDir()
				a := NewPatcherWithTempDir(patcherDir, keepOldDir)
				err = a.PatchFilesystem(outDir, bytes.NewReader(tt.args.patch), tt.args.expected)
				if (err != nil) != tt.wantErr {
					t.Fatalf("PatchFilesystem() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				readDir, err := os.ReadDir(patcherDir)
				if err != nil {
					t.Fatal(err)
				}
				if len(readDir) != 0 {
					t.Fatal("patcher did not clean up temp dir")
				}
				eq, _ := fileutils.CompareDirectories(outDir, expectedDir)
				if !eq {
					t.Fatalf("output directory does not match expected directory")
				}
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
