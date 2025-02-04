package fileutils

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestReplaceFile verifies that ReplaceFile atomically renames the current file to target.
func TestReplaceFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create target file with initial content.
	targetPath := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(targetPath, []byte("old"), 0644); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}

	// Create current file with new content.
	currentPath := filepath.Join(tmpDir, "current.txt")
	if err := os.WriteFile(currentPath, []byte("new"), 0644); err != nil {
		t.Fatalf("failed to write current file: %v", err)
	}

	// Replace target file with current file.
	if err := ReplaceFile(currentPath, targetPath); err != nil {
		t.Fatalf("ReplaceFile failed: %v", err)
	}

	// Verify target file now has the new content.
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}
	if string(data) != "new" {
		t.Errorf("expected target file content 'new', got '%s'", string(data))
	}

	// Verify that the current file has been moved.
	if _, err := os.Stat(currentPath); !os.IsNotExist(err) {
		t.Errorf("expected current file to be removed, but it exists")
	}
}

// TestReplaceDirectory verifies that ReplaceDirectory atomically replaces a target directory.
func TestReplaceDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a target directory with initial content.
	targetPath := filepath.Join(tmpDir, "targetDir")
	if err := os.Mkdir(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target directory: %v", err)
	}
	targetFile := filepath.Join(targetPath, "file.txt")
	if err := os.WriteFile(targetFile, []byte("old"), 0644); err != nil {
		t.Fatalf("failed to write file in target directory: %v", err)
	}

	// Create a current directory with new content.
	currentPath := filepath.Join(tmpDir, "currentDir")
	if err := os.Mkdir(currentPath, 0755); err != nil {
		t.Fatalf("failed to create current directory: %v", err)
	}
	currentFile := filepath.Join(currentPath, "file.txt")
	if err := os.WriteFile(currentFile, []byte("new"), 0644); err != nil {
		t.Fatalf("failed to write file in current directory: %v", err)
	}

	// Replace target directory with current directory.
	if err := ReplaceDirectory(currentPath, targetPath); err != nil {
		t.Fatalf("ReplaceDirectory failed: %v", err)
	}

	// Verify target directory now has the new content.
	data, err := os.ReadFile(filepath.Join(targetPath, "file.txt"))
	if err != nil {
		t.Fatalf("failed to read file in target directory: %v", err)
	}
	if string(data) != "new" {
		t.Errorf("expected file content 'new', got '%s'", string(data))
	}

	// Verify that the current directory no longer exists.
	if _, err := os.Stat(currentPath); !os.IsNotExist(err) {
		t.Errorf("expected current directory to be removed, but it exists")
	}
}

// TestConcurrentReplaceFile ensures that concurrent calls to ReplaceFile do not interfere.
func TestConcurrentReplaceFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.txt")

	// Create an initial target file.
	if err := os.WriteFile(targetPath, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to write initial target file: %v", err)
	}

	// Prepare two current files with different contents.
	current1 := filepath.Join(tmpDir, "current1.txt")
	current2 := filepath.Join(tmpDir, "current2.txt")
	if err := os.WriteFile(current1, []byte("first"), 0644); err != nil {
		t.Fatalf("failed to write current1 file: %v", err)
	}
	if err := os.WriteFile(current2, []byte("second"), 0644); err != nil {
		t.Fatalf("failed to write current2 file: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	// Run two replacements concurrently.
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := ReplaceFile(current1, targetPath); err != nil {
			errCh <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := ReplaceFile(current2, targetPath); err != nil {
			errCh <- err
		}
	}()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("Concurrent ReplaceFile error: %v", err)
	}

	// Verify that the final content is either "first" or "second".
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file after concurrent replace: %v", err)
	}
	content := string(data)
	if content != "first" && content != "second" {
		t.Errorf("unexpected content in target file: got '%s'", content)
	}
}

// TestConcurrentReplaceDirectory ensures that concurrent calls to ReplaceDirectory do not interfere.
func TestConcurrentReplaceDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "targetDir")

	// Create an initial target directory.
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatalf("failed to create target directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to write initial file in target directory: %v", err)
	}

	// Create two current directories with different contents.
	currentDir1 := filepath.Join(tmpDir, "currentDir1")
	currentDir2 := filepath.Join(tmpDir, "currentDir2")
	if err := os.Mkdir(currentDir1, 0755); err != nil {
		t.Fatalf("failed to create currentDir1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(currentDir1, "file.txt"), []byte("first"), 0644); err != nil {
		t.Fatalf("failed to write file in currentDir1: %v", err)
	}
	if err := os.Mkdir(currentDir2, 0755); err != nil {
		t.Fatalf("failed to create currentDir2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(currentDir2, "file.txt"), []byte("second"), 0644); err != nil {
		t.Fatalf("failed to write file in currentDir2: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	// Run two directory replacements concurrently.
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := ReplaceDirectory(currentDir1, targetDir); err != nil {
			errCh <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := ReplaceDirectory(currentDir2, targetDir); err != nil {
			errCh <- err
		}
	}()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("Concurrent ReplaceDirectory error: %v", err)
	}

	// Verify that target directory now has content from either currentDir1 or currentDir2.
	data, err := os.ReadFile(filepath.Join(targetDir, "file.txt"))
	if err != nil {
		t.Fatalf("failed to read file in target directory after concurrent replace: %v", err)
	}
	content := string(data)
	if content != "first" && content != "second" {
		t.Errorf("unexpected content in target directory: got '%s'", content)
	}
}
