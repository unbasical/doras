package ociutils

import (
	"encoding/json"
	"errors"
	"io"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/pkg/constants"
)

func ParseManifest(content io.Reader) (v1.Manifest, error) {
	var mf v1.Manifest
	err := json.NewDecoder(content).Decode(&mf)
	if err != nil {
		return v1.Manifest{}, err
	}
	return mf, nil
}

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
