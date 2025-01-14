package bsdiff

import (
	"testing"

	"github.com/unbasical/doras-server/pkg/delta"
)

func TestDiffer_Interface(t *testing.T) {
	var c any = &creator{}
	_, ok := (c).(delta.Differ)
	if !ok {
		t.Error("interface not implemented")
	}
}
