package funcutils

import (
	"fmt"
	"testing"
)

func TestMultiError(t *testing.T) {
	type args struct {
		errs []error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "No errors", args: args{errs: nil}, wantErr: false},
		{name: "Multiple nils", args: args{errs: []error{
			nil,
			nil,
		}}, wantErr: false},
		{name: "Only errors", args: args{errs: []error{
			fmt.Errorf("foo"),
			fmt.Errorf("bar"),
		}}, wantErr: true},
		{name: "Some errors", args: args{errs: []error{
			fmt.Errorf("foo"),
			nil,
		}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := MultiError(tt.args.errs...); (err != nil) != tt.wantErr {
				t.Errorf("MultiError() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMultiErrFunc(t *testing.T) {
	type args struct {
		funcs []func() error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "No errors", args: args{funcs: nil}, wantErr: false},
		{name: "Multiple nils", args: args{funcs: []func() error{
			func() error { return nil },
			func() error { return nil },
		}}, wantErr: false},
		{name: "Only errors", args: args{funcs: []func() error{
			func() error { return fmt.Errorf("foo") },
			func() error { return fmt.Errorf("bar") },
		}}, wantErr: true},
		{name: "Some errors", args: args{funcs: []func() error{
			func() error { return fmt.Errorf("foo") },
			func() error { return nil },
		}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := MultiErrFunc(tt.args.funcs...); (err != nil) != tt.wantErr {
				t.Errorf("MultiErrFunc() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
