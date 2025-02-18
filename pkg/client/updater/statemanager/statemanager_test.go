package statemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestState is a simple type used for testing.
type TestState struct {
	Value int `json:"value"`
}

// TestCommitAndLoad verifies that Commit writes the state file correctly
// and Load returns the expected state.
func TestCommitAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	// Create a Manager with an initial state.
	initialState := TestState{Value: 42}
	mgr := &Manager[TestState]{
		state: initialState,
		path:  stateFile,
	}
	if err := mgr.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Change the in-memory state and then load from disk.
	mgr.state = TestState{Value: 0}
	loadedState, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loadedState.Value != 42 {
		t.Errorf("Expected state value 42, got %d", loadedState.Value)
	}
}

// TestLoadNonExistent verifies that if the state file does not exist,
// Load returns the current in-memory state.
func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "nonexistent.json")

	defaultState := TestState{Value: 100}
	mgr := &Manager[TestState]{
		state: defaultState,
		path:  stateFile,
	}
	loadedState, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loadedState.Value != defaultState.Value {
		t.Errorf("Expected default state value %d, got %d", defaultState.Value, loadedState.Value)
	}
}

// TestLoadNonExistent verifies that if the state file does not exist,
// Load returns the current in-memory state.
func TestLoadEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "empty.json")
	_ = os.WriteFile(stateFile, []byte(""), 0644)
	defaultState := TestState{Value: 100}
	mgr := &Manager[TestState]{
		state: defaultState,
		path:  stateFile,
	}
	loadedState, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loadedState.Value != defaultState.Value {
		t.Errorf("Expected default state value %d, got %d", defaultState.Value, loadedState.Value)
	}
}

// TestConcurrentAccess simulates concurrent Commit operations to verify that
// file locking serializes access to the state file.
func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	var wg sync.WaitGroup
	numGoroutines := 10
	// expectedValues holds the values that were successfully written.
	expectedValues := make(map[int]bool)
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			mgr := &Manager[TestState]{
				state: TestState{Value: val},
				path:  stateFile,
			}
			if err := mgr.Commit(); err != nil {
				t.Errorf("Commit failed for value %d: %v", val, err)
				return
			}
			// Sleep briefly to allow overlap.
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			expectedValues[val] = true
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Load the final state.
	mgr := &Manager[TestState]{
		path: stateFile,
	}
	loadedState, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	mu.Lock()
	_, ok := expectedValues[loadedState.Value]
	mu.Unlock()
	if !ok {
		t.Errorf("Loaded state value %d not among expected values", loadedState.Value)
	}
}

// TestModifyState_Update verifies that ModifyState applies the callback modification
// and writes the updated state to disk.
func TestModifyState_Update(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	// Initialize Manager with an initial state and commit it.
	initialState := TestState{Value: 10}
	mgr := &Manager[TestState]{
		state: initialState,
		path:  stateFile,
	}
	if err := mgr.Commit(); err != nil {
		t.Fatalf("Initial Commit failed: %v", err)
	}

	// Modify the state: increment Value by 5.
	if err := mgr.ModifyState(func(s *TestState) error {
		s.Value += 5
		return nil
	}); err != nil {
		t.Fatalf("ModifyState failed: %v", err)
	}

	// Load the state from disk and verify the change.
	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	expected := 15
	if loaded.Value != expected {
		t.Errorf("Expected Value %d, got %d", expected, loaded.Value)
	}
}

// TestModifyState_CallbackError verifies that if the callback returns an error,
// the state file remains unchanged.
func TestModifyState_CallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	// Initialize Manager with an initial state and commit it.
	initialState := TestState{Value: 20}
	mgr := &Manager[TestState]{
		state: initialState,
		path:  stateFile,
	}
	if err := mgr.Commit(); err != nil {
		t.Fatalf("Initial Commit failed: %v", err)
	}

	// Modify state with a callback that returns an error.
	expectedErr := fmt.Errorf("callback error")
	err := mgr.ModifyState(func(s *TestState) error {
		s.Value = 999 // attempt to modify the state
		return expectedErr
	})
	if err == nil {
		t.Fatal("Expected error from ModifyState callback, got nil")
	}
	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error: %q, got: %q", expectedErr.Error(), err.Error())
	}

	// Load state from disk. It should remain unchanged from the initial state.
	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Value != 20 {
		t.Errorf("Expected Value 20 (unchanged), got %d", loaded.Value)
	}
}

func TestConcurrentModifyState(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")
	initialState := TestState{Value: 0}
	mgr := &Manager[TestState]{
		state: initialState,
		path:  stateFile,
	}
	if err := mgr.Commit(); err != nil {
		t.Fatalf("Initial Commit failed: %v", err)
	}

	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// use this mutex and variable so we can tell which value was written last in the case the goroutines are not ran in sequence
	m := sync.Mutex{}
	lastVal := 0
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			if err := mgr.ModifyState(func(s *TestState) error {
				m.Lock()
				defer m.Unlock()
				s.Value++
				lastVal = s.Value
				return nil
			}); err != nil {
				t.Errorf("ModifyState failed: %v", err)
			}
		}()
	}
	wg.Wait()

	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Value != lastVal {
		t.Errorf("Expected final Value %d, got %d", lastVal, loaded.Value)
	}
}
