package buildurl

import (
	"testing"
)

func TestNew(t *testing.T) {
	type args struct {
		options []Option
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "base only",
			args: args{
				options: []Option{
					WithBasePath("example.org"),
				}},
			want: "example.org",
		},
		{
			name: "one path element",
			args: args{
				options: []Option{
					WithBasePath("example.org"),
					WithPathElement("foo"),
				}},
			want: "example.org/foo",
		},
		{
			name: "multiple path elements",
			args: args{
				options: []Option{
					WithBasePath("example.org"),
					WithPathElement("foo"),
					WithPathElement("bar"),
				}},
			want: "example.org/foo/bar",
		},
		{
			name: "with single query param",
			args: args{
				options: []Option{
					WithBasePath("example.org"),
					WithQueryParam("foo", "bar"),
				}},
			want: "example.org?foo=bar",
		},
		{
			name: "with multiple query params",
			args: args{
				options: []Option{
					WithBasePath("example.org"),
					WithQueryParam("bar", "foo"),
					WithQueryParam("foo", "bar"),
				}},
			want: "example.org?bar=foo&foo=bar",
		},
		{
			name: "with repeated query param",
			args: args{
				options: []Option{
					WithBasePath("example.org"),
					WithQueryParam("foo", "1"),
					WithQueryParam("foo", "2"),
				}},
			want: "example.org?foo=1&foo=2",
		},
		{
			name: "with repeated query param",
			args: args{
				options: []Option{
					WithBasePath("example.org"),
					WithListQueryParam("foo", []string{"1", "2"}),
				}},
			want: "example.org?foo=1&foo=2",
		},
		{
			name: "one path and query param",
			args: args{
				options: []Option{
					WithBasePath("example.org"),
					WithPathElement("foo"),
					WithQueryParam("foo", "bar"),
				}},
			want: "example.org/foo?foo=bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.options...); got != tt.want {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}
