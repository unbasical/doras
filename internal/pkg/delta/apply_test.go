package delta

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/unbasical/doras-server/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"

	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
)

func TestApplyDelta_Bspatch(t *testing.T) {
	from := []byte("Hello")
	to := []byte("Hello World")
	bsDiffPatch, err := bsdiff.Bytes(from, to)
	if err != nil {
		t.Error(err)
	}
	rc, err := ApplyDelta(
		"bsdiff",
		bytes.NewReader(bsDiffPatch),
		bytes.NewReader(from),
	)
	if err != nil {
		t.Error(err)
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, false, "")
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(to, data) {
		t.Errorf("got %q, want %q", data, to)
	}
}

func TestApplyDelta_Tarpatch(t *testing.T) {
	diff := fileutils.ReadOrPanic("../../../test/test-files/delta.patch.tardiff")
	from := fileutils.ReadOrPanic("../../../test/test-files/from.tar.gz")
	to := fileutils.ReadOrPanic("../../../test/test-files/to.tar.gz")

	rc, err := ApplyDelta(
		"tardiff",
		bytes.NewReader(diff),
		bytes.NewReader(from),
	)
	if err != nil {
		t.Error(err)
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, false, "")
	gz, err := gzip.NewReader(bytes.NewReader(to))
	if err != nil {
		t.Error(err)
	}
	// the output is not compressed, so we compare it to the decompressed version of `to`
	want, err := io.ReadAll(gz)
	if err != nil {
		t.Error(err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("got %q, want %q", got, want)
	}
}
