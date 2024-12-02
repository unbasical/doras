package delta

import (
	"compress/gzip"
	"fmt"
	"github.com/unbasical/doras-server/pkg/constants"
	"io"

	"github.com/unbasical/doras-server/internal/pkg/delta/tarfsdatasource"
	"github.com/unbasical/doras-server/internal/pkg/funcutils"

	tarpatch "github.com/containers/tar-diff/pkg/tar-patch"
	"github.com/gabstv/go-bsdiff/pkg/bspatch"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func ApplyDelta(deltaKind string, diff io.Reader, content io.Reader) (io.ReadCloser, error) {
	switch deltaKind {
	case "tardiff":
		return Tarpatch(content, diff)
	case "bsdiff":
		return Bspatch(content, diff)
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

func Bspatch(old io.Reader, patch io.Reader) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		err := bspatch.Reader(old, pw, patch)
		if err != nil {
			errInner := pw.CloseWithError(err)
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(errInner), false, "failed to close pipe writer after error")
		}
		funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")
	}()
	return pr, nil
}
func Tarpatch(old io.Reader, patch io.Reader) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	dataSource := tarfsdatasource.New(old, func(reader io.Reader) io.Reader {
		gzr, err := gzip.NewReader(reader)
		if err != nil {
			panic(err)
		}
		return gzr
	})
	go func() {
		err := tarpatch.Apply(patch, dataSource, pw)
		if err != nil {
			errInner := pw.CloseWithError(err)
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(errInner), false, "failed to close pipe writer after error")
		}
		funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")
	}()
	return pr, nil
}
