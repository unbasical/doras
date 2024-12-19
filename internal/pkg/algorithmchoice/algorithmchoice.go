package algorithmchoice

import (
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/compression/zstd"
	"slices"

	"github.com/unbasical/doras-server/pkg/constants"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/compression/gzip"
	"github.com/unbasical/doras-server/internal/pkg/delta/bsdiff"
	"github.com/unbasical/doras-server/internal/pkg/delta/tardiff"
	"github.com/unbasical/doras-server/internal/pkg/utils/compressionutils"

	"github.com/unbasical/doras-server/pkg/compression"
	"github.com/unbasical/doras-server/pkg/delta"
)

type AlgorithmChoice struct {
	delta.Differ
	compression.Compressor
}

func (c *AlgorithmChoice) GetTag() string {
	if compressorName := c.Compressor.Name(); compressorName != "" {
		return c.Differ.Name() + "_" + compressorName
	}
	return c.Differ.Name()
}

func (c *AlgorithmChoice) GetMediaType() string {
	if compressorName := c.Compressor.Name(); compressorName != "" {
		return "application/" + c.Differ.Name() + "+" + compressorName
	}
	return "application/" + c.Differ.Name()
}

func (c *AlgorithmChoice) GetFileExt() string {
	if compressorName := c.Compressor.Name(); compressorName != "" {
		return fmt.Sprintf(".%s.%s", c.Differ.Name(), compressorName)
	}
	return fmt.Sprintf(".%s", c.Differ.Name())
}

func ChooseAlgorithm(acceptedAlgorithms []string, mfFrom, mfTo *v1.Manifest) AlgorithmChoice {
	_ = mfTo

	algorithm := AlgorithmChoice{
		Differ:     bsdiff.NewCreator(),
		Compressor: compressionutils.NewNopCompressor(),
	}
	if mfFrom.Layers[0].Annotations[constants.ContentUnpack] == "true" && slices.Contains(acceptedAlgorithms, "tardiff") {
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
