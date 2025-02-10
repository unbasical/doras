package bsdiff

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/pkg/algorithm/delta"

	bsdiff2 "github.com/gabstv/go-bsdiff/pkg/bsdiff"
)

func TestPatcher_Patch(t *testing.T) {
	from := []byte("Hello")
	to := []byte("Hello World")
	bsDiffPatch, err := bsdiff2.Bytes(from, to)
	patcher := NewPatcher()
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
			got, _ := patcher.Patch(tt.args.old, tt.args.patch)
			data, err := io.ReadAll(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("bspatch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (err != nil) && tt.wantErr {
				return
			}

			if !bytes.Equal(data, tt.want) {
				t.Errorf("bspatch()\ngot = %v,\n want %v", data, tt.want)
			}
		})
	}
}

func TestPatcher_PatchFilesystem(t *testing.T) {
	from := []byte("Hello")
	to := []byte("Hello World")
	bsDiffPatch, err := bsdiff2.Bytes(from, to)

	if err != nil {
		t.Error(err)
	}
	type args struct {
		old      io.Reader
		patch    io.Reader
		expected *digest.Digest
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "success (without digest)", args: args{
				old:   bytes.NewReader(from),
				patch: bytes.NewReader(bsDiffPatch),
			},
			want:    to,
			wantErr: false,
		},
		{
			name: "success (with digest)", args: args{
				old:      bytes.NewReader(from),
				patch:    bytes.NewReader(bsDiffPatch),
				expected: func() *digest.Digest { d := digest.FromBytes(to); return &d }(),
			},
			want:    to,
			wantErr: false,
		},
		{
			name: "failure (bad digest)", args: args{
				old:      bytes.NewReader(from),
				patch:    bytes.NewReader(bsDiffPatch),
				expected: func() *digest.Digest { d := digest.FromBytes(nil); return &d }(),
			},
			want:    to,
			wantErr: true,
		},
		{
			name: "failure (bad patch)", args: args{
				old:   bytes.NewReader(from),
				patch: bytes.NewReader([]byte{}),
			},
			want:    to,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldFile, err := os.CreateTemp(t.TempDir(), "bsdiff-test-*")
			if err != nil {
				t.Fatal(err)
			}
			oldData, err := io.ReadAll(tt.args.old)
			if err != nil {
				t.Fatal(err)
			}
			_, err = io.Copy(oldFile, bytes.NewReader(oldData))
			if err != nil {
				t.Fatal(err)
			}
			workingDir := t.TempDir()
			patcher := NewPatcherWithTempDir(workingDir)
			err = patcher.PatchFilesystem(oldFile.Name(), tt.args.patch, tt.args.expected)
			got := fileutils.ReadOrPanic(oldFile.Name())
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("unexpected error: %v", err)
				}
				if !bytes.Equal(oldData, got) {
					t.Fatal("old file was modified despite error")
				}
				return
			}
			readDir, err := os.ReadDir(workingDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(readDir) != 0 {
				t.Fatal("expected no files to exist in temp dir")
			}
			if !bytes.Equal(tt.want, got) {
				t.Fatalf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

func TestPatcher_Interface(t *testing.T) {
	var c any = &patcher{}
	_, ok := (c).(delta.Patcher)
	if !ok {
		t.Error("interface not implemented")
	}
}
