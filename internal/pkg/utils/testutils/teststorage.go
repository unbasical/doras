package testutils

import (
	"bytes"
	"context"
	"fmt"
	"github.com/unbasical/doras/pkg/constants"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
)

// FileDescription represents a file that is stored as an OCI image.
type FileDescription struct {
	// Name of the file.
	Name string
	// Data of the file.
	Data []byte
	// MediaType of the file.
	MediaType string
	// Tag of the file.
	Tag string
	// NeedsUnpack indicates if it is an archived file or not.
	NeedsUnpack bool
}

// StorageFromFiles creates an oras.ReadOnlyTarget that stores the given files.
func StorageFromFiles(ctx context.Context, rootDir string, files []FileDescription) (oras.ReadOnlyTarget, error) {
	store, err := oci.New(rootDir)
	if err != nil {
		return nil, err
	}

	// 1. Add files to the file store
	for _, f := range files {
		fileDescriptors := make([]v1.Descriptor, 0, len(files))

		dgst, err := digest.FromReader(bytes.NewReader(f.Data))
		if err != nil {
			return nil, err
		}
		d := v1.Descriptor{
			MediaType: f.MediaType,
			Digest:    dgst,
			Size:      int64(len(f.Data)),
			Annotations: map[string]string{
				constants.OciImageTitle:       f.Name,
				"io.deis.oras.content.unpack": fmt.Sprintf("%v", f.NeedsUnpack),
			},
		}

		if d.MediaType == "" {
			d.MediaType = "application/vnd.test.file"
		}

		err = store.Push(ctx, d, bytes.NewReader(f.Data))
		if err != nil {
			return nil, fmt.Errorf("failed to add file to storage: %w", err)
		}
		fileDescriptors = append(fileDescriptors, d)
		// 2. Pack the files and tag the packed manifest
		artifactType := "application/vnd.test.artifact"
		opts := oras.PackManifestOptions{
			Layers: fileDescriptors,
		}
		manifestDescriptor, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, artifactType, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to pack manifest: %w", err)
		}

		if err = store.Tag(ctx, manifestDescriptor, f.Tag); err != nil {
			return nil, fmt.Errorf("failed to tag manifest: %w", err)
		}
	}

	return store, nil
}
