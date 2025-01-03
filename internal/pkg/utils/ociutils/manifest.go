package ociutils

import (
	"encoding/json"
	"errors"
	"io"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/pkg/constants"
	"oras.land/oras-go/v2"
)

func GetDeltaManifest(fromImage v1.Descriptor, toImage v1.Descriptor, layers []v1.Descriptor, algo string) oras.PackManifestOptions {
	fromDigest := "sha256:" + fromImage.Digest.Encoded()
	toDigest := "sha256:" + toImage.Digest.Hex()
	opts := oras.PackManifestOptions{
		Layers: layers,
		ManifestAnnotations: map[string]string{
			constants.DorasAnnotationFrom:      fromDigest,
			constants.DorasAnnotationTo:        toDigest,
			constants.DorasAnnotationAlgorithm: algo,
		},
	}
	return opts
}

// GetDeltaDummyManifest produces a manifest for a dummy manifest.
// Its metadata specifies it is a dummy, and it has a single empty layer.
func GetDeltaDummyManifest(fromImage v1.Descriptor, toImage v1.Descriptor, algo string) oras.PackManifestOptions {
	fromDigest := "sha256:" + fromImage.Digest.Encoded()
	toDigest := "sha256:" + toImage.Digest.Hex()
	opts := oras.PackManifestOptions{
		Layers: []v1.Descriptor{v1.DescriptorEmptyJSON},
		ManifestAnnotations: map[string]string{
			constants.DorasAnnotationFrom:      fromDigest,
			constants.DorasAnnotationTo:        toDigest,
			constants.DorasAnnotationAlgorithm: algo,
			constants.DorasAnnotationIsDummy:   "true",
		},
	}
	return opts
}

func ParseManifest(content io.Reader) (v1.Manifest, error) {
	var mf v1.Manifest
	err := json.NewDecoder(content).Decode(&mf)
	if err != nil {
		return v1.Manifest{}, err
	}
	return mf, nil
}

func ExtractPathFromManifest(mf *v1.Manifest) (path string, isArchive bool, err error) {
	if unpack, ok := mf.Annotations[constants.ContentUnpack]; ok && unpack == "true" {
		isArchive = true
	}
	path, ok := mf.Annotations["org.opencontainers.image.title"]
	if !ok {
		return "", isArchive, errors.New("missing file title")
	}
	return path, isArchive, nil
}
