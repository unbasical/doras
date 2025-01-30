package bsdiff

import (
	"bytes"
	bsdiff2 "github.com/gabstv/go-bsdiff/pkg/bsdiff"
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"io"
	"strings"
	"testing"
)

func TestDiffer_Interface(t *testing.T) {
	var c any = &differ{}
	_, ok := (c).(delta.Differ)
	if !ok {
		t.Error("interface not implemented")
	}
}

func TestCreator_Diff(t *testing.T) {
	differ := NewDiffer()
	from := "hello"
	to := "world"
	expected, err := bsdiff2.Bytes([]byte(from), []byte(to))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := differ.Diff(strings.NewReader(from), strings.NewReader(to))
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
