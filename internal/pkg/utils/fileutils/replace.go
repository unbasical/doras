package fileutils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path"
	"syscall"
)

// getLockFile computes a unique lock file path based on newPath.
func getLockFile(newPath string) string {
	hash := sha256.Sum256([]byte(newPath))
	return path.Join(os.TempDir(), "update_lock_"+hex.EncodeToString(hash[:]))
}

// acquireLock opens (or creates) the specified lock file and acquires an exclusive lock.
func acquireLock(lockPath string) (*os.File, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// releaseLock releases the file lock and closes the file.
func releaseLock(f *os.File) error {
	return errors.Join(
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN),
		f.Close(),
	)
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
		if err := os.RemoveAll(targetPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(currentPath, targetPath)
}
