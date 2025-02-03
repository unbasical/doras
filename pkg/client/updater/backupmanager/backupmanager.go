package backupmanager

import (
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/pkg/algorithm/compression"
	"path"
)

// BackupManager handles storing old versions and creating diffs.
type BackupManager interface {
	CreateBackup(currentVersion, oldPath, newPath string) error
	RestoreBackup(oldVersion, oldPath, newPath string) error
}

type compressedBackup struct {
	backupDir string
	comp      compression.Compressor
	decomp    compression.Decompressor
}

// NewCompressedBackupManager creates backups made of compressed tars.
func NewCompressedBackupManager(backupDir string, comp compression.Compressor) BackupManager {
	return &compressedBackup{
		backupDir: backupDir,
		comp:      comp,
	}
}

type backupState struct {
	Algo    string `json:"algo"`
	Version string `json:"version"`
}

func (c *compressedBackup) CreateBackup(currentVersion, oldPath, _ string) error {
	// TODO:
	//   - check if a backup for the current version exists
	//   - if it does not exist, check if a backup was started for the current version
	//   - if it was started check if it is valid
	//   - write backup as attempted backup if no valid backup exists
	state := backupState{
		Algo:    c.comp.Name(),
		Version: currentVersion,
	}
	err := fileutils.SafeWriteJson(path.Join(c.backupDir, "attempted.json"), &state)
	if err != nil {
		return err
	}
	// TODO:
	// 	- create compressed tar and write it while hashing
	// 	- Use safe writing with sync wrapper
	// 	- move attempted to current backup.
	panic("implement me")
}

func (c *compressedBackup) RestoreBackup(oldVersion, oldPath, newPath string) error {
	// TODO:
	// 	1. Create decompressed/extracted directory in a temp dir.
	//  2. Sync to disk
	//  3. Swap directories and and delete old directory.
	panic("implement me")
}
