package validator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
)

// ManifestValidator defines an interface for validating an OCI manifest against specific criteria or rules.
type ManifestValidator interface {
	Validate(desc *v1.Descriptor, mf *ociutils.Manifest) error
}

// SizeLimitedValidator ensures that the size of an artifact does not exceed a specified limit.
// Limit specifies the maximum allowed size in bytes.
type SizeLimitedValidator struct {
	Limit uint64
}

// Validate checks if the total size of the artifact does not exceed the configured size limit.
// It returns an error if the size exceeds the limit or nil otherwise.
func (s SizeLimitedValidator) Validate(desc *v1.Descriptor, mf *ociutils.Manifest) error {
	err := checkSizeLimit(desc, mf, 0, s.Limit)
	if err != nil {
		return fmt.Errorf("artifact size exceeds configured limit: %w", err)
	}
	return nil
}

// VolumeLimitValidator validates volume usage against a specified limit within a given time period and directory.
type VolumeLimitValidator struct {
	StatsDir string
	Limit    uint64
	Period   time.Duration
}

func (v VolumeLimitValidator) consumedVolume() (uint64, error) {
	return SumUpDownloadStats(v.StatsDir, v.Period)
}

// SumUpDownloadStats calculates the sum of uint64 values from files in statsDir modified within the given period.
// Files older than the period are deleted, and errors during operations are returned where applicable.
// The opposite function is inspector.WriteUintToFile.
// nolint:revive // pass cognitive complexity lint, in my opinion the complexity of this function is acceptable
func SumUpDownloadStats(statsDir string, period time.Duration) (uint64, error) {
	entries, err := os.ReadDir(statsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read directory %s: %w", statsDir, err)
	}

	now := time.Now().UTC()
	var sum uint64

	for _, entry := range entries {
		fullPath := filepath.Join(statsDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return 0, fmt.Errorf("failed to get info for file %s: %w", fullPath, err)
		}

		// Calculate file age
		age := now.Sub(info.ModTime().UTC())

		// Delete files older than maxAge and files from the future
		if age > period || age < 0 {
			if err := os.Remove(fullPath); err != nil {
				return sum, fmt.Errorf("failed to delete old file %s: %w", fullPath, err)
			}
			continue
		}
		// Read and parse the file's uint64 value
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return sum, fmt.Errorf("failed to read file %s: %w", fullPath, err)
		}
		value, err := strconv.ParseUint(string(data), 10, 64)
		if err != nil {
			return sum, fmt.Errorf("failed to parse uint64 from file %s: %w", fullPath, err)
		}
		sum += value
	}
	return sum, nil
}

// Validate checks if the combined size of the descriptor and manifest exceeds the configured volume limit.
func (v VolumeLimitValidator) Validate(desc *v1.Descriptor, mf *ociutils.Manifest) error {
	consumed, err := v.consumedVolume()
	if err != nil {
		return err
	}
	err = checkSizeLimit(desc, mf, consumed, v.Limit)
	if err != nil {
		return fmt.Errorf("exceeded download volume limit (%v/%v): %w", v.Limit, v.Period, err)
	}
	return nil
}

func checkSizeLimit(desc *v1.Descriptor, mf *ociutils.Manifest, baseSize, limit uint64) error {
	artifactSize := uint64(desc.Size)
	for _, l := range slices.Concat(mf.Layers, mf.Blobs) {
		artifactSize += uint64(l.Size)
	}
	if artifactSize+baseSize > limit {
		return fmt.Errorf("artifact + base size (%d + %d bytes) surpasses limit (%d bytes)", artifactSize, baseSize, limit)
	}
	return nil
}

// WriteUintToFile creates a file in the specified directory with the current UTC Unix time
// as its name and writes the provided uint64 value into it.
// The opposite function is validator.SumUpDownloadStats.
func WriteUintToFile(dir string, value uint64) error {
	// Ensure the directory exists (creates it if necessary)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to ensure directory %s: %w", dir, err)
	}

	// Use the current UTC Unix timestamp as the filename
	timestamp := uint64(time.Now().UTC().Unix())
	filename := strconv.FormatUint(timestamp, 10)
	fullPath := filepath.Join(dir, filename)
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	n, err := f.WriteString(strconv.FormatUint(value, 10))
	if err != nil {
		return err
	}
	return errors.Join(
		f.Truncate(int64(n)),
		f.Sync(),
	)
}
