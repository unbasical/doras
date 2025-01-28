package ociutils

import (
	"errors"
	"github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/pkg/constants"
)

// ExtractPathFromManifest returns the path information and whether the artifact contains an archive.
// The information is expected to be stored in the Manifest's annotations.
func ExtractPathFromManifest(mf *Manifest) (path string, isArchive bool, err error) {
	if unpack, ok := mf.Annotations[constants.OrasContentUnpack]; ok && unpack == "true" {
		isArchive = true
	}
	path, ok := mf.Annotations["org.opencontainers.image.title"]
	if !ok {
		return "", isArchive, errors.New("missing file title")
	}
	return path, isArchive, nil
}

// Manifest provides `application/vnd.oci.image.manifest.v1+json` mediatype structure when marshalled to JSON.
type Manifest struct {
	specs.Versioned

	// MediaType specifies the type of this document data structure e.g. `application/vnd.oci.image.manifest.v1+json`
	MediaType string `json:"mediaType,omitempty"`

	// ArtifactType specifies the IANA media type of artifact when the manifest is used for an artifact.
	ArtifactType string `json:"artifactType,omitempty"`

	// Config references a configuration object for a container, by digest.
	// The referenced configuration object is a JSON blob that the runtime uses to set up the container.
	Config v1.Descriptor `json:"config"`

	// Layers is an indexed list of layers referenced by the manifest.
	Layers []v1.Descriptor `json:"layers"`

	// Blobs is an indexed list of blobs referenced by the manifest.
	Blobs []v1.Descriptor `json:"blobs"`

	// Subject is an optional link from the image manifest to another manifest forming an association between the image manifest and the other manifest.
	Subject *v1.Descriptor `json:"subject,omitempty"`

	// Annotations contains arbitrary metadata for the image manifest.
	Annotations map[string]string `json:"annotations,omitempty"`
}
