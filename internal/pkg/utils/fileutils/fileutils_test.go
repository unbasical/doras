package fileutils

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// createTempDirWithFiles creates a temporary directory with specified files and contents
func createTempDirWithFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp(t.TempDir(), "test-dir")
	if err != nil {
		t.Fatal(err)
	}

	for path, content := range files {
		filePath := filepath.Join(dir, path)
		dirPath := filepath.Dir(filePath)

		// Create parent directories if needed
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatal(err)
		}

		// Write file content
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestCompareDirectories(t *testing.T) {
	t.Run("Identical Directories", func(t *testing.T) {
		files := map[string]string{
			"file1.txt":        "hello",
			"subdir/file2.txt": "world",
		}
		dir1 := createTempDirWithFiles(t, files)
		defer func() {
			_ = os.RemoveAll(dir1)
		}()
		dir2 := createTempDirWithFiles(t, files)
		defer func() {
			_ = os.RemoveAll(dir2)
		}()

		equal, err := CompareDirectoriesHasher(dir1, dir2, sha256.New())
		if err != nil || !equal {
			t.Errorf("Expected directories to be identical, got unequal: %v", err)
		}
	})

	t.Run("Different File Content", func(t *testing.T) {
		files1 := map[string]string{
			"file1.txt": "hello",
		}
		files2 := map[string]string{
			"file1.txt": "different content",
		}
		dir1 := createTempDirWithFiles(t, files1)
		defer func() {
			_ = os.RemoveAll(dir1)
		}()
		dir2 := createTempDirWithFiles(t, files2)
		defer func() {
			_ = os.RemoveAll(dir2)
		}()

		equal, err := CompareDirectoriesHasher(dir1, dir2, sha256.New())
		if err != nil {
			t.Fatal(err)
		}
		if equal {
			t.Errorf("Expected directories to be different, but they were reported as equal")
		}
	})

	t.Run("Missing File", func(t *testing.T) {
		files1 := map[string]string{
			"file1.txt": "hello",
			"file2.txt": "world",
		}
		files2 := map[string]string{
			"file1.txt": "hello",
		}
		dir1 := createTempDirWithFiles(t, files1)
		defer func() {
			_ = os.RemoveAll(dir1)
		}()
		dir2 := createTempDirWithFiles(t, files2)
		defer func() {
			_ = os.RemoveAll(dir2)
		}()

		equal, err := CompareDirectoriesHasher(dir1, dir2, sha256.New())
		if err != nil {
			t.Fatal(err)
		}
		if equal {
			t.Errorf("Expected directories to be different due to missing file, but they were reported as equal")
		}
	})

	t.Run("Extra File", func(t *testing.T) {
		files1 := map[string]string{
			"file1.txt": "hello",
		}
		files2 := map[string]string{
			"file1.txt": "hello",
			"file2.txt": "world",
		}
		dir1 := createTempDirWithFiles(t, files1)
		defer func() {
			_ = os.RemoveAll(dir1)
		}()
		dir2 := createTempDirWithFiles(t, files2)
		defer func() {
			_ = os.RemoveAll(dir2)
		}()

		equal, err := CompareDirectoriesHasher(dir1, dir2, sha256.New())
		if err != nil {
			t.Fatal(err)
		}
		if equal {
			t.Errorf("Expected directories to be different due to extra file, but they were reported as equal")
		}
	})
}
