package fileutils

import (
	"testing"
)

func TestEnsureSubPath(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		rel     string
		want    string
		wantErr bool
	}{
		{name: "simple", base: "/a/b", rel: "c.txt", want: "/a/b/c.txt"},
		{name: "nested", base: "/a/b", rel: "c/d/e.txt", want: "/a/b/c/d/e.txt"},
		{name: "dot segment normalised", base: "/a/b", rel: "c/./d.txt", want: "/a/b/c/d.txt"},
		{name: "traversal rejected", base: "/a/b", rel: "../c.txt", wantErr: true},
		{name: "deep traversal rejected", base: "/a/b", rel: "c/../../d.txt", wantErr: true},
		{name: "absolute rejected", base: "/a/b", rel: "/etc/passwd", wantErr: true},
		{name: "empty rejected", base: "/a/b", rel: "", wantErr: true},
		{name: "dot resolves to base", base: "/a/b", rel: ".", want: "/a/b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnsureSubPath(tt.base, tt.rel)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
