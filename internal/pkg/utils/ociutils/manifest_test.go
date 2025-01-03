package ociutils

import (
	"errors"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/unbasical/doras-server/pkg/constants"
)

func TestExtractPathFromManifest(t *testing.T) {
	tests := []struct {
		name            string
		manifest        *v1.Manifest
		expectedPath    string
		expectedArchive bool
		expectedErr     error
	}{
		{
			name: "Valid manifest with archive",
			manifest: &v1.Manifest{
				Annotations: map[string]string{
					constants.ContentUnpack:          "true",
					"org.opencontainers.image.title": "file.txt",
				},
			},
			expectedPath:    "file.txt",
			expectedArchive: true,
			expectedErr:     nil,
		},
		{
			name: "Valid manifest without archive",
			manifest: &v1.Manifest{
				Annotations: map[string]string{
					"org.opencontainers.image.title": "file.txt",
				},
			},
			expectedPath:    "file.txt",
			expectedArchive: false,
			expectedErr:     nil,
		},
		{
			name: "Missing file title",
			manifest: &v1.Manifest{
				Annotations: map[string]string{
					constants.ContentUnpack: "true",
				},
			},
			expectedPath:    "",
			expectedArchive: true,
			expectedErr:     errors.New("missing file title"),
		},
		{
			name: "Empty annotations",
			manifest: &v1.Manifest{
				Annotations: map[string]string{},
			},
			expectedPath:    "",
			expectedArchive: false,
			expectedErr:     errors.New("missing file title"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path, isArchive, err := ExtractPathFromManifest(test.manifest)
			assert.Equal(t, test.expectedPath, path)
			assert.Equal(t, test.expectedArchive, isArchive)
			if test.expectedErr != nil {
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
