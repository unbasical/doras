package fileutils

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/gofrs/flock"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

// getLockFile computes a unique lock file path based on the canonical absolute path of newPath.
func getLockFile(newPath string) string {
	abs, err := filepath.Abs(newPath)
	if err != nil {
		abs = newPath // Fallback to the provided path if an error occurs.
	}
	abs = filepath.Clean(abs)
	hash := sha256.Sum256([]byte(abs))
	return filepath.Join(os.TempDir(), "update_lock_"+hex.EncodeToString(hash[:]))
}

// acquireLock creates a new flock based on lockPath and acquires an exclusive lock.
func acquireLock(lockPath string) (*flock.Flock, error) {
	lock := flock.New(lockPath)
	// Block until the lock is acquired
	if err := lock.Lock(); err != nil {
		return nil, err
	}
	return lock, nil
}

// releaseLock releases the lock held by the flock.
func releaseLock(lock *flock.Flock) error {
	return lock.Unlock()
}

// ReplaceFile atomically replaces the file at targetPath with the file at currentPath,
// using a unique lock file based on targetPath.
func ReplaceFile(currentPath, targetPath string) error {
	lockFile := getLockFile(targetPath)
	lock, err := acquireLock(lockFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = releaseLock(lock)
	}()
	return os.Rename(currentPath, targetPath)
}

// ReplaceDirectory atomically replaces the directory at targetPath with the directory at currentPath.
// It removes any existing directory at currentPath and uses a unique lock file based on targetPath.
func ReplaceDirectory(currentPath, targetPath string) error {
	lockFile := getLockFile(targetPath)
	lock, err := acquireLock(lockFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = releaseLock(lock)
	}()
	if _, err := os.Stat(targetPath); err == nil {
		log.Debugf("removing %q", targetPath)
		if err := os.RemoveAll(targetPath); err != nil {
			log.WithError(err).Debug("failed to remove old directory")
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(currentPath, targetPath)
}
