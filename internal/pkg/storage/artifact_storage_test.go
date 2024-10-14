package storage

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/artifact"
	"os"
	"path"
	"testing"
)

func TestFilesystemStorage_LoadArtifact(t *testing.T) {
	tempDir := t.TempDir()
	log.Debug(tempDir)
	storage := FilesystemStorage{tempDir}
	filePath := path.Join(tempDir, "hello.in")
	expected := []byte("hello world")
	err := os.WriteFile(filePath, expected, 0600)
	if err != nil {
		t.Fatal(err)
	}
	artfct, err := storage.LoadArtifact("hello.in")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(artfct.GetBytes(), []byte("hello world")) {
		t.Fatalf("expected '%s' but got '%s'", string(expected), string(artfct.GetBytes()))
	}
}

func TestFilesystemStorage_StoreArtifact(t *testing.T) {
	tempDir := t.TempDir()
	storage := FilesystemStorage{tempDir}
	filePath := path.Join(tempDir, "hello.out")
	expected := []byte("hello world")
	bytesArtifact := artifact.RawBytesArtifact{
		Data: expected,
	}
	err := storage.StoreArtifact(&bytesArtifact, "hello.out")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytesArtifact.Data, data) {
		t.Fatalf("expected '%s' but got %s", string(expected), string(data))
	}
}
