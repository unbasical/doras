package ociutils

import (
	"errors"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/pkg/constants"
)

// ExtractPathFromManifest returns the path information and whether the artifact contains an archive.
// The information is expected to be stored in the Manifest's annotations.
func ExtractPathFromManifest(mf *v1.Manifest) (path string, isArchive bool, err error) {
	if unpack, ok := mf.Annotations[constants.OrasContentUnpack]; ok && unpack == "true" {
		isArchive = true
	}
	path, ok := mf.Annotations["org.opencontainers.image.title"]
	if !ok {
		return "", isArchive, errors.New("missing file title")
	}
	return path, isArchive, nil
}
