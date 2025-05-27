package validator

import (
	"errors"
	"fmt"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/internal/pkg/utils/observer"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync/atomic"
	"time"
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
	return checkSizeLimit(desc, mf, 0, s.Limit)
}

// VolumeLimitValidator validates volume usage against a specified limit within a given time period and directory.
type VolumeLimitValidator struct {
	StatsDir string
	Limit    uint64
	Period   time.Duration
}

func (v VolumeLimitValidator) consumedVolume() (uint64, error) {
	entries, err := os.ReadDir(v.StatsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read directory %s: %w", v.StatsDir, err)
	}

	now := time.Now().UTC()
	var sum uint64

	for _, entry := range entries {
		fullPath := filepath.Join(v.StatsDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return 0, fmt.Errorf("failed to get info for file %s: %w", fullPath, err)
		}

		// Calculate file age
		age := now.Sub(info.ModTime().UTC())

		// Delete files older than maxAge
		if age > v.Period {
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
	return checkSizeLimit(desc, mf, consumed, v.Limit)
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

// ObserveDownloadStats periodically observes a uint64 atomic value and writes it to a file in the specified directory.
// dir specifies the directory where the observation results are stored.
// p is a pointer to the atomic uint64 value to be observed.
// stop is a channel to signal when the periodic observation should stop.
// Returns an error if any occurs during the observation or file writing process.
func ObserveDownloadStats(dir string, p *atomic.Uint64, stop <-chan any) error {
	o := observer.IntervalObserver[*atomic.Uint64]{
		Interval: funcutils.Unwrap(time.ParseDuration("15s")),
		F: func(p *atomic.Uint64) error {
			loaded := p.Load()
			return WriteUintToFile(dir, loaded)
		},
		Observable: p,
	}
	return o.Observe(stop)
}
