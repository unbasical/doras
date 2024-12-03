package delta

import (
	"fmt"

	"io"

	"github.com/unbasical/doras-server/internal/pkg/delta/tardiff"

	"github.com/unbasical/doras-server/internal/pkg/delta/bsdiff"
	"github.com/unbasical/doras-server/pkg/constants"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func ApplyDelta(deltaKind string, diff io.Reader, content io.Reader) (io.ReadCloser, error) {
	switch deltaKind {
	case "tardiff":
		r, err := (&tardiff.Applier{}).Apply(content, diff)
		return io.NopCloser(r), err
	case "bsdiff":
		r, err := (&bsdiff.Applier{}).Apply(content, diff)
		return io.NopCloser(r), err
	default:
		return nil, fmt.Errorf("unsupported delta algorithm: %q", deltaKind)
	}
}

func ApplyDeltaWithBlobDescriptor(blobDescriptor v1.Descriptor, diff io.Reader, content io.Reader) (io.ReadCloser, error) {
	name, ok := blobDescriptor.Annotations["org.opencontainers.image.title"]
	if !ok || name == "" {
		return nil, fmt.Errorf("missing file name in annotations: %v", blobDescriptor.Annotations)
	}
	algorithm, ok := blobDescriptor.Annotations[constants.DorasAnnotationAlgorithm]
	if !ok || algorithm == "" {
		return nil, fmt.Errorf("missing delta algorithm in annotations: %v", blobDescriptor.Annotations)
	}
	return ApplyDelta(algorithm, diff, content)
}
