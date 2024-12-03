package bsdiff

import (
	"bytes"
	"io"
	"testing"

	bsdiff2 "github.com/gabstv/go-bsdiff/pkg/bsdiff"

	"github.com/unbasical/doras-server/pkg/delta"
)

func TestApplier_Apply(t *testing.T) {
	from := []byte("Hello")
	to := []byte("Hello World")
	bsDiffPatch, err := bsdiff2.Bytes(from, to)
	patcher := &Applier{}
	if err != nil {
		t.Error(err)
	}
	type args struct {
		old   io.Reader
		patch io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{name: "success", args: args{
			old:   bytes.NewReader(from),
			patch: bytes.NewReader(bsDiffPatch),
		}, want: to, wantErr: false},
		{name: "error", args: args{
			old:   bytes.NewReader(from),
			patch: bytes.NewReader([]byte{}),
		}, want: to, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := patcher.Apply(tt.args.old, tt.args.patch)
			data, err := io.ReadAll(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bspatch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (err != nil) && tt.wantErr {
				return
			}

			if !bytes.Equal(data, tt.want) {
				t.Errorf("Bspatch()\ngot = %v,\n want %v", data, tt.want)
			}
		})
	}
}

func TestApplier_Interface(t *testing.T) {
	var c any = &Applier{}
	_, ok := (c).(delta.Applier)
	if !ok {
		t.Error("interface not implemented")
	}
}
