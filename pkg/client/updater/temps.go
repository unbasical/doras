package updater

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/pkg/constants"
)

// internalDirTempPrefixes are the prefixes of temporary entries created directly
// under the internal directory during pull operations.
var internalDirTempPrefixes = []string{
	constants.TempPrefixDeltas,
	constants.TempPrefixIntermediate,
	constants.TempPrefixExtract,
}

// patcherDirTempPrefixes are the prefixes of temporary entries created under the
// patcher directory while applying deltas.
var patcherDirTempPrefixes = []string{
	constants.TempPrefixTarpatch,
	constants.TempPrefixTarExtract,
	constants.TempPrefixBsdiff,
}

// cleanupStaleTempArtifacts removes leftover temporary files and directories from
// previous pull operations that were interrupted (e.g. by a reboot, an OOM kill,
// or a disk-full error) before their in-function deferred cleanup could run.
//
// Cleanup only ever removes entries whose names match one of the well-known temp
// prefixes (see pkg/constants). Persistent state is never touched: the state file
// (doras-state.json), its lock file, the fetcher download cache and the patcher
// directory itself are all left in place.
//
// This assumes a single client owns a given internal directory at a time (the way
// the updater is used in practice). If that assumption is ever relaxed so that
// multiple clients share one internal directory concurrently, this prefix-only
// sweep could race a live patch and would need an age guard.
//
// The cleanup is best-effort: individual removal failures are logged and skipped
// rather than aborting client initialisation.
func cleanupStaleTempArtifacts(internalDir, patcherDir string) {
	removed := removeMatchingEntries(internalDir, internalDirTempPrefixes)
	removed += removeMatchingEntries(patcherDir, patcherDirTempPrefixes)
	if removed > 0 {
		log.Infof("removed %d stale temporary artifact(s) from previous interrupted pulls", removed)
	}
}

// removeMatchingEntries removes every entry in dir whose name starts with one of
// the provided prefixes. It returns the number of entries that were removed.
// Errors reading the directory or removing individual entries are logged and do
// not stop the sweep.
func removeMatchingEntries(dir string, prefixes []string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Warnf("failed to scan %q for stale temporary artifacts", dir)
		}
		return 0
	}
	var removed int
	for _, entry := range entries {
		name := entry.Name()
		if !hasAnyPrefix(name, prefixes) {
			continue
		}
		fullPath := filepath.Join(dir, name)
		if err := os.RemoveAll(fullPath); err != nil {
			log.WithError(err).Warnf("failed to remove stale temporary artifact %q", fullPath)
			continue
		}
		log.Debugf("removed stale temporary artifact %q", fullPath)
		removed++
	}
	return removed
}

// hasAnyPrefix reports whether name starts with any of the provided prefixes.
func hasAnyPrefix(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
