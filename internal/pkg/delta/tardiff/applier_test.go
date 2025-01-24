package tardiff

import (
	"bytes"
	"compress/gzip"
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"io"
	"testing"

	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
)

func TestApplier_Apply(t *testing.T) {
	applier := &applier{}

	diff := fileutils.ReadOrPanic("../../../../test/test-files/delta.patch.tardiff")
	from := fileutils.ReadOrPanic("../../../../test/test-files/from.tar.gz")
	to := fileutils.ReadOrPanic("../../../../test/test-files/to.tar.gz")

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
			patch: bytes.NewReader(diff),
		}, want: to, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applier.Patch(tt.args.old, tt.args.patch)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bspatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && tt.wantErr {
				return
			}
			data, err := io.ReadAll(got)
			if err != nil {
				t.Error(err)
			}
			gzr, err := gzip.NewReader(bytes.NewReader(tt.want))
			if err != nil {
				t.Error(err)
			}
			if err != nil {
				t.Error(err)
			}
			want, err := io.ReadAll(gzr)
			if err != nil {
				t.Error(err)
			}

			if !bytes.Equal(data, want) {
				t.Errorf("Bspatch()\ngot = %v,\n want %v", data, want)
			}
		})
	}
}

func TestPatcher_Interface(t *testing.T) {
	var c any = &applier{}
	_, ok := (c).(delta.Patcher)
	if !ok {
		t.Error("interface not implemented")
	}
}
