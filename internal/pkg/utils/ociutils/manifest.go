package ociutils

import (
	"encoding/json"
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
