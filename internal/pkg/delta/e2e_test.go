package delta

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/unbasical/doras-server/internal/pkg/fileutils"
	"github.com/unbasical/doras-server/internal/pkg/funcutils"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func Test_BsdiffDeltaE2E(t *testing.T) {
	from := "Hello"
	to := "Hello World!"
	fromDigest, err := digest.FromReader(strings.NewReader("from"))
	if err != nil {
		t.Fatal(err)
	}
	fromDescriptor := v1.Descriptor{
		Digest: fromDigest,
		Size:   int64(len(from)),
		Annotations: map[string]string{
			"org.opencontainers.image.title": "foo",
		},
	}
	toDigest, err := digest.FromReader(strings.NewReader("to"))
	if err != nil {
		t.Fatal(err)
	}
	toDescriptor := v1.Descriptor{
		Digest: toDigest,
		Size:   int64(len(to)),
		Annotations: map[string]string{
			"org.opencontainers.image.title": "foo",
		},
	}
	ext, rc, err := createDelta(fromDescriptor, toDescriptor, strings.NewReader(from), strings.NewReader(to))
	if err != nil {
		t.Error(err)
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, true, "failed to close reader")
	if ext != "bsdiff" {
		t.Error("wrong delta type")
	}
	deltaDescriptor := v1.Descriptor{
		Annotations: map[string]string{
			"org.opencontainers.image.title": "foo.patch." + ext,
		},
	}
	rc, err = ApplyDelta(deltaDescriptor, rc, strings.NewReader(from))
	if err != nil {
		t.Error(err)
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, true, "failed to close reader")
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Error(err)
	}
	if string(got) != to {
		t.Errorf("got %q, want %q", got, to)
	}
}

func Test_TardiffDeltaE2E(t *testing.T) {
	from := fileutils.ReadOrPanic("test-files/from.tar.gz")
	to := fileutils.ReadOrPanic("test-files/to.tar.gz")
	fromDigest, err := digest.FromReader(strings.NewReader("from"))
	if err != nil {
		t.Fatal(err)
	}
	fromDescriptor := v1.Descriptor{
		Digest: fromDigest,
		Size:   int64(len(from)),
		Annotations: map[string]string{
			"org.opencontainers.image.title": "foo",
		},
	}
	toDigest, err := digest.FromReader(strings.NewReader("to"))
	if err != nil {
		t.Fatal(err)
	}
	toDescriptor := v1.Descriptor{
		Digest: toDigest,
		Size:   int64(len(to)),
		Annotations: map[string]string{
			"org.opencontainers.image.title": "foo",
		},
	}
	ext, rc, err := createDelta(fromDescriptor, toDescriptor, bytes.NewReader(from), bytes.NewReader(to))
	if err != nil {
		t.Error(err)
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, true, "failed to close reader")
	if ext != "bsdiff" {
		t.Error("wrong delta type")
	}
	deltaDescriptor := v1.Descriptor{
		Annotations: map[string]string{
			"org.opencontainers.image.title": "foo.patch." + ext,
		},
	}
	rc, err = ApplyDelta(deltaDescriptor, rc, bytes.NewReader(from))
	if err != nil {
		t.Error(err)
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, true, "failed to close reader")
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Error(err)
	}
	gzr, err := gzip.NewReader(bytes.NewBuffer(to))
	if err != nil {
		t.Error(err)
	}
	want, err := io.ReadAll(gzr)
	if err != nil {
		t.Error(err)
	}
	if bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
