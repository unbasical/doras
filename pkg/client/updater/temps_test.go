package updater

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/unbasical/doras/pkg/constants"
)

// writeFile creates a file with some content, failing the test on error.
func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("failed to write %q: %v", path, err)
	}
}

// mkdir creates a directory, failing the test on error.
func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("failed to mkdir %q: %v", path, err)
	}
}

func TestCleanupStaleTempArtifacts(t *testing.T) {
	internalDir := t.TempDir()
	patcherDir := filepath.Join(internalDir, "patcher-dir")
	mkdir(t, patcherDir)

	// Stale temp entries under the internal directory (mix of files and dirs).
	staleInternal := []string{
		constants.TempPrefixDeltas + "123",
		constants.TempPrefixIntermediate + "abc",
		constants.TempPrefixExtract + "xyz",
	}
	for _, name := range staleInternal {
		// exercise both file and directory removal
		p := filepath.Join(internalDir, name)
		mkdir(t, p)
		writeFile(t, filepath.Join(p, "inner"))
	}

	// Stale temp entries under the patcher directory.
	staleTarpatchFile := filepath.Join(patcherDir, constants.TempPrefixTarpatch+"999")
	writeFile(t, staleTarpatchFile)
	staleTarExtractDir := filepath.Join(patcherDir, constants.TempPrefixTarExtract+"999")
	mkdir(t, staleTarExtractDir)
	staleBsdiffFile := filepath.Join(patcherDir, constants.TempPrefixBsdiff+"999")
	writeFile(t, staleBsdiffFile)

	// Protected entries that must survive.
	stateFile := filepath.Join(internalDir, "doras-state.json")
	writeFile(t, stateFile)
	lockFile := filepath.Join(internalDir, "doras-state.json.lock")
	writeFile(t, lockFile)
	fetcherDir := filepath.Join(internalDir, "fetcher")
	mkdir(t, filepath.Join(fetcherDir, "completed"))
	fetcherBlob := filepath.Join(fetcherDir, "completed", "blob")
	writeFile(t, fetcherBlob)
	unrelatedFile := filepath.Join(internalDir, "something-else")
	writeFile(t, unrelatedFile)
	// A patcher-dir entry that does not match any temp prefix must also survive.
	patcherKeep := filepath.Join(patcherDir, "keep-me")
	writeFile(t, patcherKeep)

	cleanupStaleTempArtifacts(internalDir, patcherDir)

	// All stale temp entries must be gone.
	gone := []string{
		filepath.Join(internalDir, staleInternal[0]),
		filepath.Join(internalDir, staleInternal[1]),
		filepath.Join(internalDir, staleInternal[2]),
		staleTarpatchFile,
		staleTarExtractDir,
		staleBsdiffFile,
	}
	for _, p := range gone {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected stale temp %q to be removed, stat err = %v", p, err)
		}
	}

	// All protected entries must still exist.
	kept := []string{
		stateFile,
		lockFile,
		fetcherDir,
		fetcherBlob,
		unrelatedFile,
		patcherDir,
		patcherKeep,
	}
	for _, p := range kept {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected protected entry %q to survive, stat err = %v", p, err)
		}
	}
}

// TestCleanupStaleTempArtifacts_MissingDirs ensures the sweep is a no-op (no
// panic, no error surfaced) when the directories do not exist.
func TestCleanupStaleTempArtifacts_MissingDirs(t *testing.T) {
	base := t.TempDir()
	cleanupStaleTempArtifacts(
		filepath.Join(base, "does-not-exist"),
		filepath.Join(base, "does-not-exist", "patcher-dir"),
	)
}

// TestNewClientCleansStaleTempArtifacts verifies the sweep is wired into client
// construction: pre-existing patcher-dir/internal-dir temps are removed while the
// fetcher cache is preserved.
func TestNewClientCleansStaleTempArtifacts(t *testing.T) {
	internalDir := t.TempDir()
	outputDir := t.TempDir()
	patcherDir := filepath.Join(internalDir, "patcher-dir")
	mkdir(t, patcherDir)

	staleTarpatch := filepath.Join(patcherDir, constants.TempPrefixTarpatch+"stale")
	writeFile(t, staleTarpatch)
	staleDeltas := filepath.Join(internalDir, constants.TempPrefixDeltas+"stale")
	mkdir(t, staleDeltas)

	// Fetcher cache content that must be preserved across construction.
	fetcherBlob := filepath.Join(internalDir, "fetcher", "completed", "blob")
	mkdir(t, filepath.Dir(fetcherBlob))
	writeFile(t, fetcherBlob)

	_, err := NewClient(
		WithInternalDirectory(internalDir),
		WithOutputDirectory(outputDir),
		WithRemoteURL("http://localhost:0"),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if _, err := os.Stat(staleTarpatch); !os.IsNotExist(err) {
		t.Errorf("expected stale tarpatch temp to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(staleDeltas); !os.IsNotExist(err) {
		t.Errorf("expected stale deltas temp to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(fetcherBlob); err != nil {
		t.Errorf("expected fetcher cache to survive, stat err = %v", err)
	}
}
