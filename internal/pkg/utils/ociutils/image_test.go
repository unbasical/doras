package ociutils

import "testing"

func TestIsDigest(t *testing.T) {
	type args struct {
		imageOrTag string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "image tagged with digest", args: args{imageOrTag: "abca@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}, want: true},
		{name: "tag with digest", args: args{imageOrTag: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}, want: true},
		{name: "image with tag", args: args{imageOrTag: "hello:latest"}, want: false},
		{name: "tag only", args: args{imageOrTag: "latest"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDigest(tt.args.imageOrTag); got != tt.want {
				t.Errorf("IsDigest() = %v, want %v", got, tt.want)
			}
		})
	}
}
