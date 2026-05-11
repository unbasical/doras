package fileutils

import (
	"fmt"
	"path/filepath"
	"strings"
)

// EnsureSubPath validates that joining base and rel produces a path contained
// within base. It rejects empty paths, absolute paths, and paths that traverse
// outside of base (e.g. using ".."). Returns the cleaned joined path.
func EnsureSubPath(base, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("empty relative path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%q is an absolute path", rel)
	}
	joined := filepath.Join(base, rel)
	cleanBase := filepath.Clean(base) + string(filepath.Separator)
	cleanJoined := filepath.Clean(joined)
	if !strings.HasPrefix(cleanJoined+string(filepath.Separator), cleanBase) {
		return "", fmt.Errorf("path %q escapes base directory %q", rel, base)
	}
	if cleanJoined == filepath.Clean(base) {
		return "", fmt.Errorf("path %q resolves to base directory", rel)
	}
	return cleanJoined, nil
}
