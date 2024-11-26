package delta

import (
	"compress/gzip"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/delta/tarfsdatasource"
	"github.com/unbasical/doras-server/internal/pkg/funcutils"
	"io"
	"strings"

	tarpatch "github.com/containers/tar-diff/pkg/tar-patch"
	"github.com/gabstv/go-bsdiff/pkg/bspatch"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func ApplyDelta(target v1.Descriptor, diff io.Reader, content io.Reader) (io.ReadCloser, error) {
	name, ok := target.Annotations["org.opencontainers.image.title"]
	if !ok || name == "" {
		return nil, fmt.Errorf("missing file name in annotations: %v", target.Annotations)
	}
	split := strings.Split(name, ".")
	if len(split) < 2 {
		return nil, fmt.Errorf("invalid file name, missing extension: %q", name)
	}
	fileExtension := split[len(split)-1]
	switch fileExtension {
	case "tardiff":
		return Tarpatch(content, diff)
	case "bsdiff":
		return Bspatch(content, diff)
	default:
		return nil, fmt.Errorf("unsupported delta algorithm: %q", fileExtension)
	}
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
