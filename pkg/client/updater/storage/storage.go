package storage

import "github.com/unbasical/doras/pkg/client/updater/backupmanager"

type Storage interface {
	DeltaDirectory() string
	IntermediateDirectory() string
	ArtifactDirectory() string
	BackupDirectory() string
	CurrentVersion() (string, bool)
}

// Updater manages applying updates and rollbacks.
type Updater interface {
	ApplyUpdate(outDir, newArtifactPath string) error
	Rollback(b backupmanager.BackupManager) error
	MarkFailed(version string)
	HasFailed(version string) bool
}
