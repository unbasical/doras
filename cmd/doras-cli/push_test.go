package main

import (
	"context"
	"oras.land/oras-go/v2"
	"reflect"
	"testing"
)

func Test_cliArgs_getCompressor(t *testing.T) {

	tests := []struct {
		name           string
		compressorName string
		want           string
		wantErr        bool
	}{
		{name: "success (zstd)", compressorName: "zstd", want: "zstd", wantErr: false},
		{name: "success (gzip)", compressorName: "gzip", want: "gzip", wantErr: false},
		{name: "success (none)", compressorName: "none", want: "", wantErr: false},
		{name: "empty name", compressorName: "", want: "", wantErr: true},
		{name: "unknown name", compressorName: "foo", want: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := &cliArgs{}
			args.Push.Compress = tt.compressorName
			got, err := args.getCompressor()
			if (err != nil) != tt.wantErr {
				t.Errorf("getCompressor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(got.Name(), tt.want) {
				t.Errorf("getCompressor() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_pushDirectory(t *testing.T) {
	type args struct {
		ctx    context.Context
		args   *cliArgs
		target oras.Target
		tag    string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := pushDirectory(tt.args.ctx, tt.args.args, tt.args.target, tt.args.tag); (err != nil) != tt.wantErr {
				t.Errorf("pushDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_pushFile(t *testing.T) {
	type args struct {
		ctx    context.Context
		args   *cliArgs
		target oras.Target
		tag    string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := pushFile(tt.args.ctx, tt.args.args, tt.args.target, tt.args.tag); (err != nil) != tt.wantErr {
				t.Errorf("pushFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
