package tardiff

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	gzip2 "github.com/unbasical/doras/internal/pkg/compression/gzip"
	"github.com/unbasical/doras/internal/pkg/utils/tarutils"
	"github.com/unbasical/doras/pkg/algorithm/delta"

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

type tarEntry struct {
	name    string
	content string
	dir     bool
}

func writeTarGz(t *testing.T, path string, entries []tarEntry) {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0644, Typeflag: tar.TypeReg, Size: int64(len(e.content))}
		if e.dir {
			hdr = &tar.Header{Name: e.name + "/", Mode: 0755, Typeflag: tar.TypeDir}
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if !e.dir {
			if _, err := tw.Write([]byte(e.content)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func Test_patcher_PatchFilesystem_NestedDirectories(t *testing.T) {
	from := []tarEntry{
		{name: "top.txt", content: "top v1"},
		{name: "changed", dir: true},
		{name: "changed/file.txt", content: "changed v1"},
		{name: "changed/sub", dir: true},
		{name: "changed/sub/deep.txt", content: "deep v1"},
		{name: "unchanged", dir: true},
		{name: "unchanged/file.txt", content: "unchanged"},
		{name: "removed", dir: true},
		{name: "removed/file.txt", content: "removed"},
		{name: "file-to-dir", content: "was a file"},
		{name: "dir-to-file", dir: true},
		{name: "dir-to-file/file.txt", content: "was a dir"},
	}
	to := []tarEntry{
		{name: "top.txt", content: "top v2"},
		{name: "changed", dir: true},
		{name: "changed/file.txt", content: "changed v2"},
		{name: "changed/sub", dir: true},
		{name: "changed/sub/deep.txt", content: "deep v2"},
		{name: "changed/sub/added.txt", content: "added"},
		{name: "unchanged", dir: true},
		{name: "unchanged/file.txt", content: "unchanged"},
		{name: "file-to-dir", dir: true},
		{name: "file-to-dir/file.txt", content: "now a dir"},
		{name: "dir-to-file", content: "now a file"},
		{name: "added", dir: true},
		{name: "added/file.txt", content: "added"},
	}
	fixtures := t.TempDir()
	fromPath := fixtures + "/from.tar.gz"
	toPath := fixtures + "/to.tar.gz"
	writeTarGz(t, fromPath, from)
	writeTarGz(t, toPath, to)

	expectedDir := t.TempDir()
	if err := tarutils.ExtractCompressedTar(expectedDir, "", toPath, nil, gzip2.NewDecompressor()); err != nil {
		t.Fatal(err)
	}

	for _, keepOldDir := range []bool{true, false} {
		t.Run(fmt.Sprintf("keepOldDir=%t", keepOldDir), func(t *testing.T) {
			fromFp, err := os.Open(fromPath)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = fromFp.Close() }()
			toFp, err := os.Open(toPath)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = toFp.Close() }()
			patch, err := NewCreator().Diff(fromFp, toFp)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = patch.Close() }()

			outDir := t.TempDir()
			if err := tarutils.ExtractCompressedTar(outDir, "", fromPath, nil, gzip2.NewDecompressor()); err != nil {
				t.Fatal(err)
			}

			patcherDir := t.TempDir()
			before, err := os.Stat(outDir)
			if err != nil {
				t.Fatal(err)
			}
			keptDirBefore, err := os.Stat(filepath.Join(outDir, "changed", "sub"))
			if err != nil {
				t.Fatal(err)
			}
			a := NewPatcherWithTempDir(patcherDir, keepOldDir, 0755)
			if err := a.PatchFilesystem(outDir, patch, nil); err != nil {
				t.Fatalf("PatchFilesystem() error = %v", err)
			}
			eq, err := fileutils.CompareDirectories(outDir, expectedDir)
			if err != nil {
				t.Fatal(err)
			}
			if !eq {
				t.Fatal("output directory does not match expected directory")
			}
			if !keepOldDir {
				return
			}
			after, err := os.Stat(outDir)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) {
				t.Error("output directory was replaced, expected it to be kept")
			}
			keptDirAfter, err := os.Stat(filepath.Join(outDir, "changed", "sub"))
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(keptDirBefore, keptDirAfter) {
				t.Error("nested directory was replaced, expected it to be kept")
			}
			if _, err := os.Lstat(filepath.Join(outDir, "removed")); !os.IsNotExist(err) {
				t.Errorf("expected removed directory to be gone, got err = %v", err)
			}
			if info, err := os.Lstat(filepath.Join(outDir, "dir-to-file")); err != nil || !info.Mode().IsRegular() {
				t.Errorf("expected dir-to-file to be a regular file, got info = %v, err = %v", info, err)
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
				err = os.Chmod(outDir, 0755)
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

				a := NewPatcherWithTempDir(patcherDir, keepOldDir, 0777)
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
				stat, err := os.Stat(outDir)
				if err != nil {
					t.Fatal(err)
				}
				expectedPerms := 0777 | fs.ModeDir
				if tt.wantErr == false && stat.Mode() != expectedPerms {
					t.Fatalf("output directory permissions do not match expected permission: got=%v, expected=%v", stat.Mode(), expectedPerms)
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
