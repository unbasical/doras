package algorithmchoice

import (
	"fmt"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"slices"

	"github.com/unbasical/doras/internal/pkg/compression/zstd"

	"github.com/unbasical/doras/pkg/constants"

	"github.com/unbasical/doras/internal/pkg/compression/gzip"
	"github.com/unbasical/doras/internal/pkg/delta/bsdiff"
	"github.com/unbasical/doras/internal/pkg/delta/tardiff"
	"github.com/unbasical/doras/internal/pkg/utils/compressionutils"

	"github.com/unbasical/doras/pkg/algorithm/compression"
)

// PatcherChoice aggregates the two algorithms that are used to apply a delta patch.
// The decompressor can be a null compressor which does no decompression at all.
// For the diffing side refer to DifferChoice.
type PatcherChoice struct {
	delta.Patcher
	compression.Decompressor
}

// DifferChoice aggregates the two algorithms that are used to create a delta patch.
// // The compressor can be a null compressor which does no compression at all.
// For the patching side refer to PatcherChoice.
type DifferChoice struct {
	delta.Differ
	compression.Compressor
}

// GetTagSuffix returns the suffix that is added to the tag of the delta image to identify the used algorithms.
func (c *DifferChoice) GetTagSuffix() string {
	if compressorName := c.Compressor.Name(); compressorName != "" {
		return c.Differ.Name() + "_" + compressorName
	}
	return c.Differ.Name()
}

// GetMediaType returns the media type that is used to identify the algorithms of a delta patch.
// For instance, a zstd-compressed bsdiff patch has the value `application/bsdiff+zstd`.
func (c *DifferChoice) GetMediaType() string {
	if compressorName := c.Compressor.Name(); compressorName != "" {
		return "application/" + c.Differ.Name() + "+" + compressorName
	}
	return "application/" + c.Differ.Name()
}

// GetFileExt returns the file extension that is appended to the file name of a delta patch.
// For instance, a zstd-compressed bsdiff patch has the value `.bsdiff.zstd`.
func (c *DifferChoice) GetFileExt() string {
	if compressorName := c.Compressor.Name(); compressorName != "" {
		return fmt.Sprintf(".%s.%s", c.Differ.Name(), compressorName)
	}
	return fmt.Sprintf(".%s", c.Differ.Name())
}

// ChooseAlgorithms returns a DifferChoice that is the most suitable to create a delta patch for the given artifacts,
// under the constraint of only using the acceptedAlgorithms.
func ChooseAlgorithms(acceptedAlgorithms []string, mfFrom, mfTo *ociutils.Manifest) DifferChoice {
	_ = mfTo

	algorithm := DifferChoice{
		Differ:     bsdiff.NewCreator(),
		Compressor: compressionutils.NewNopCompressor(),
	}
	var artifacts []v1.Descriptor
	if len(mfFrom.Layers) == 1 {
		artifacts = mfFrom.Layers
	}
	if len(mfTo.Blobs) == 1 {
		artifacts = mfTo.Blobs
	}
	if artifacts[0].Annotations[constants.OrasContentUnpack] == "true" && slices.Contains(acceptedAlgorithms, "tardiff") {
		algorithm.Differ = tardiff.NewCreator()
	}
	// The order is inverse to the priority.
	if slices.Contains(acceptedAlgorithms, "gzip") {
		algorithm.Compressor = gzip.NewCompressor()
	}
	if slices.Contains(acceptedAlgorithms, "zstd") {
		algorithm.Compressor = zstd.NewCompressor()
	}
	return algorithm
}
