package bsdiff

import (
	"io"
	"reflect"
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

func TestCreator_Create(t *testing.T) {
	type args struct {
		old io.Reader
		new io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    io.Reader
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &creator{}
			got, err := c.Diff(tt.args.old, tt.args.new)
			if (err != nil) != tt.wantErr {
				t.Errorf("Diff() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Diff() got = %v, want %v", got, tt.want)
			}
		})
	}
}
