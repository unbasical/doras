package tardiff

import (
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"testing"
)

func TestDiffer_Interface(t *testing.T) {
	var c any = &Creator{}
	_, ok := (c).(delta.Differ)
	if !ok {
		t.Error("interface not implemented")
	}
}
