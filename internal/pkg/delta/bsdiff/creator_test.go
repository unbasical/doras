package bsdiff

import (
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"testing"
)

func TestDiffer_Interface(t *testing.T) {
	var c any = &creator{}
	_, ok := (c).(delta.Differ)
	if !ok {
		t.Error("interface not implemented")
	}
}
