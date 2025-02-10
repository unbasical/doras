package tarutils

import (
	"archive/tar"
	"errors"
	"fmt"
	"github.com/opencontainers/go-digest"
	"github.com/unbasical/doras/pkg/algorithm/compression"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractCompressedTar decompresses the gzip
// and extracts tar file to a directory specified by the `dir` parameter.
func ExtractCompressedTar(dir, prefix, filename string, checksum *digest.Digest, decom compression.Decompressor) (err error) {
	fp, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := fp.Close()
		if err == nil {
			err = closeErr
		}
	}()
	var verifier digest.Verifier
	var r io.Reader = fp
	if checksum != nil {
		verifier = checksum.Verifier()
		r = io.TeeReader(r, verifier)
	}
	r, err = decom.Decompress(r)
	if err != nil {
		return err
	}

	if err := extractTarDirectory(dir, prefix, r); err != nil {
		return err
	}
	if verifier != nil && !verifier.Verified() {
		return errors.New("content digest mismatch")
	}
	return nil
}

// extractTarDirectory extracts tar file to a directory specified by the `dir`
// parameter. The file name prefix is ensured to be the string specified by the
// `prefix` parameter and is trimmed.
func extractTarDirectory(dir, prefix string, r io.Reader) error {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		// Name check
		name := header.Name
		path, err := ensureBasePath(dir, prefix, name)
		if err != nil {
			return err
		}
		path = filepath.Join(dir, path)

		// Create content
		switch header.Typeflag {
		case tar.TypeReg:
			err = writeFile(path, tr, header.FileInfo().Mode())
		case tar.TypeDir:
			err = os.MkdirAll(path, header.FileInfo().Mode())
		default:
			return fmt.Errorf("unsupported file type %v", header.Typeflag)
		}
		if err != nil {
			return err
		}

		// Change access time and modification time if possible (error ignored)
		_ = os.Chtimes(path, header.AccessTime, header.ModTime)
	}
}

// ensureBasePath ensures the target path is in the base path,
// returning its relative path to the base path.
// target can be either an absolute path or a relative path.
func ensureBasePath(baseAbs, baseRel, target string) (string, error) {
	base := baseRel
	if filepath.IsAbs(target) {
		// ensure base and target are consistent
		base = baseAbs
	}
	path, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("%q is outside of %q", target, baseRel)
	}

	// No symbolic link allowed in the relative path
	dir := filepath.Dir(path)
	for dir != "." {
		if info, err := os.Lstat(filepath.Join(baseAbs, dir)); err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
		} else if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("no symbolic link allowed between %q and %q", baseRel, target)
		}
		dir = filepath.Dir(dir)
	}

	return path, nil
}

// writeFile writes content to the file specified by the `path` parameter.
func writeFile(path string, r io.Reader, perm os.FileMode) (err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}()

	_, err = io.Copy(file, r)
	// call Sync to make sure file is written to the disk
	return errors.Join(err, file.Sync())
}
