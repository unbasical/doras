package delta

import (
	"fmt"

	"io"

	"github.com/unbasical/doras-server/internal/pkg/delta/tardiff"

	"github.com/unbasical/doras-server/internal/pkg/delta/bsdiff"
)

func ApplyDelta(deltaKind string, diff io.Reader, content io.Reader) (io.ReadCloser, error) {
	switch deltaKind {
	case "tardiff":
		r, err := (&tardiff.Applier{}).Patch(content, diff)
		return io.NopCloser(r), err
	case "bsdiff":
		r, err := (&bsdiff.Applier{}).Patch(content, diff)
		return io.NopCloser(r), err
	default:
		return nil, fmt.Errorf("unsupported delta algorithm: %q", deltaKind)
	}
}
