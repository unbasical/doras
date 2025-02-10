package ociutils

import (
	"testing"

	"github.com/unbasical/doras/pkg/constants"
)

func TestExtractPathFromManifest(t *testing.T) {
	tests := []struct {
		name            string
		manifest        *Manifest
		expectedPath    string
		expectedArchive bool
		expectedErr     bool
	}{
		{
			name: "Valid manifest with archive",
			manifest: &Manifest{
				Annotations: map[string]string{
					constants.OrasContentUnpack: "true",
					constants.OciImageTitle:     "file.txt",
				},
			},
			expectedPath:    "file.txt",
			expectedArchive: true,
			expectedErr:     false,
		},
		{
			name: "Valid manifest without archive",
			manifest: &Manifest{
				Annotations: map[string]string{
					constants.OciImageTitle: "file.txt",
				},
			},
			expectedPath:    "file.txt",
			expectedArchive: false,
			expectedErr:     false,
		},
		{
			name: "Missing file title",
			manifest: &Manifest{
				Annotations: map[string]string{
					constants.OrasContentUnpack: "true",
				},
			},
			expectedPath:    "",
			expectedArchive: true,
			expectedErr:     true,
		},
		{
			name: "Empty annotations",
			manifest: &Manifest{
				Annotations: map[string]string{},
			},
			expectedPath:    "",
			expectedArchive: false,
			expectedErr:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path, isArchive, err := ExtractPathFromManifest(test.manifest)
			if test.expectedPath != path {
				t.Errorf("Expected: %s, Got: %s", test.expectedPath, path)
			}
			if test.expectedArchive != isArchive {
				t.Errorf("Expected: %t, Got: %t", test.expectedArchive, isArchive)
			}
			if test.expectedErr != (err != nil) {
				t.Errorf("Expected err: %v, Got: %v", test.expectedErr, err)
			}
		})
	}
}
