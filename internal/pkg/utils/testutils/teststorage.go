package testutils

import (
	"bytes"
	"context"
	"fmt"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
)

type FileDescription struct {
	Name        string
	Data        []byte
	MediaType   string
	Tag         string
	NeedsUnpack bool
}

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
				"org.opencontainers.image.title": f.Name,
				"io.deis.oras.content.unpack":    fmt.Sprintf("%v", f.NeedsUnpack),
			},
		}

		if d.MediaType == "" {
			d.MediaType = "application/vnd.test.file"
		}

		err = store.Push(ctx, d, bytes.NewReader(f.Data))
		if err != nil {
			return nil, fmt.Errorf("failed to add file to storage: %v", err)
		}
		fileDescriptors = append(fileDescriptors, d)
		// 2. Pack the files and tag the packed manifest
		artifactType := "application/vnd.test.artifact"
		opts := oras.PackManifestOptions{
			Layers: fileDescriptors,
		}
		manifestDescriptor, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, artifactType, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to pack manifest: %v", err)
		}

		if err = store.Tag(ctx, manifestDescriptor, f.Tag); err != nil {
			return nil, fmt.Errorf("failed to tag manifest: %v", err)
		}
	}

	return store, nil
}
