package tardiff

import (
	"github.com/unbasical/doras-server/pkg/delta"
	"io"
	"reflect"
	"testing"
)

func TestCreator_Interface(t *testing.T) {
	var c any = &Creator{}
	_, ok := (c).(delta.Creator)
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
			c := &Creator{}
			got, err := c.Create(tt.args.old, tt.args.new)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Create() got = %v, want %v", got, tt.want)
			}
		})
	}
}
