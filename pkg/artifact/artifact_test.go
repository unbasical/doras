package artifact

import (
	"bytes"
	"os"
	"path"
	"testing"
)

func TestFilesystemArtifact_LoadArtifact(t *testing.T) {
	tempDir := t.TempDir()
	filePath := path.Join(tempDir, "hello.in")
	expected := []byte("hello world")
	err := os.WriteFile(filePath, expected, 0644)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := LoadArtifact(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(artifact.Data, []byte("hello world")) {
		t.Fatalf("expected '%s' but got '%s'", string(expected), string(artifact.Data))
	}
}

func TestFilesystemArtifact_GetArtifact(t *testing.T) {
	tempDir := t.TempDir()
	filePath := path.Join(tempDir, "hello.out")
	expected := []byte("hello world")
	artifact := RawBytesArtifact{
		Data: expected,
	}
	err := artifact.StoreArtifact(filePath)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(artifact.Data, data) {
		t.Fatalf("expected '%s' but got %s", string(expected), string(data))
	}
}
