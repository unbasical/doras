package tardiff

import (
	"bytes"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"io"
	"testing"
)

func TestDiffer_Interface(t *testing.T) {
	var c any = &Creator{}
	_, ok := (c).(delta.Differ)
	if !ok {
		t.Error("interface not implemented")
	}
}

func TestCreator_Diff(t *testing.T) {
	differ := NewCreator()
	from := fileutils.ReadOrPanic("../../../../test/test-files/from.tar.gz")
	to := fileutils.ReadOrPanic("../../../../test/test-files/to.tar.gz")
	expected := fileutils.ReadOrPanic("../../../../test/test-files/delta.patch.tardiff")
	rc, err := differ.Diff(bytes.NewReader(from), bytes.NewReader(to))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, expected) {
		t.Errorf("differ.Diff() = %x, want %x", got, expected)
	}
	_ = rc.Close()
}
