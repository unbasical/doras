package statemanager

import (
	"encoding/json"
	"errors"
	"github.com/gofrs/flock"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
)

// Manager is a generic wrapper around a state object T which is serialized to the storage as JSON.
// It provides ways to safely mutate the state, backed by file locks.
type Manager[T any] struct {
	state T
	path  string
}

// New initializes a state manager with the provided state and overwrites existing state.
func New[T any](initialState T, p string) (*Manager[T], error) {
	m := Manager[T]{
		state: initialState,
		path:  p,
	}
	err := m.Commit()
	if err != nil {
		log.WithError(err).Debug("failed to initialize state")
		return nil, err
	}
	return &m, nil
}

// NewFromDisk initializes a state manager with the state that is exists on disk,
// if nothing is found on the disk it uses the provided default.
func NewFromDisk[T any](defaultState T, path string) (*Manager[T], error) {
	m := Manager[T]{
		state: defaultState,
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
	return errors.Join(fp.Sync(), fp.Close())
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
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			// Return current state if file is empty.
			return &m.state, nil
		}
		var syntaxError *json.SyntaxError
		if errors.As(err, &syntaxError) {
			return &m.state, nil
		}
		return nil, err
	}
	// call sync to make sure it is written to the disk
	if err = errors.Join(fp.Sync(), fp.Close()); err != nil {
		return nil, err
	}
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
		oldState := m.state
		err = json.NewDecoder(fp).Decode(&m.state)
		if err != nil {
			// cover cases where state file is empty
			var syntaxError *json.SyntaxError
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.As(err, &syntaxError) {
				_ = fp.Close()
				return err
			}
			m.state = oldState
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
	// call sync to make sure it is written to the disk
	return errors.Join(fp.Sync(), fp.Close())
}
