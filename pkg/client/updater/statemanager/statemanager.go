package statemanager

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/gofrs/flock"
)

type Manager[T any] struct {
	state T
	path  string
}

func New[T any](initialState T, path string) (*Manager[T], error) {
	m := Manager[T]{
		state: initialState,
		path:  path,
	}
	_, err := m.Load()
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Commit acquires an exclusive lock, then atomically writes the current state to the file.
func (m *Manager[T]) Commit() error {
	// Create a unique lock file path based on m.path.
	lockPath := m.path + ".lock"
	fileLock := flock.New(lockPath)
	// Acquire an exclusive lock.
	if err := fileLock.Lock(); err != nil {
		return err
	}
	defer func() {
		_ = fileLock.Unlock()
	}()

	fp, err := os.OpenFile(m.path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	// Encode state to the file.
	err = json.NewEncoder(fp).Encode(m.state)
	if err != nil {
		_ = fp.Close()
		return err
	}
	_ = fp.Close()
	return nil
}

// Load acquires a shared lock, then reads and decodes the state from the file.
func (m *Manager[T]) Load() (*T, error) {
	lockPath := m.path + ".lock"
	fileLock := flock.New(lockPath)
	// Acquire a shared (read) lock.
	if err := fileLock.RLock(); err != nil {
		return nil, err
	}
	defer func() {
		_ = fileLock.Unlock()
	}()

	fp, err := os.Open(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Return current state if file does not exist.
			return &m.state, nil
		}
		return nil, err
	}
	err = json.NewDecoder(fp).Decode(&m.state)
	if err != nil {
		_ = fp.Close()
		return nil, err
	}
	_ = fp.Close()
	return &m.state, nil
}

// ModifyState acquires an exclusive lock, loads the current state.
// It then calls the callback function on the state to modify it before writing back to disk.
func (m *Manager[T]) ModifyState(cb func(*T) error) error {
	lockPath := m.path + ".lock"
	fileLock := flock.New(lockPath)
	if err := fileLock.Lock(); err != nil {
		return err
	}
	defer func() {
		_ = fileLock.Unlock()
	}()
	// attempt to load the current state from disk, use memory state if we can nots
	fp, err := os.Open(m.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		err = json.NewDecoder(fp).Decode(&m.state)
		if err != nil {
			_ = fp.Close()
			return err
		}
	}
	err = cb(&m.state)
	if err != nil {
		return err
	}
	fp, err = os.OpenFile(m.path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	// Encode state to the file.
	err = json.NewEncoder(fp).Encode(m.state)
	if err != nil {
		_ = fp.Close()
		return err
	}
	_ = fp.Close()
	return nil
}
