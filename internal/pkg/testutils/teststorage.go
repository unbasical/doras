package testutils

import (
	"context"
	"fmt"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"io"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"os"
	"path"
)

type FileDescription struct {
	Data      io.Reader
	MediaType string
}

func StorageFromFiles(ctx context.Context, rootDir string, files map[string]FileDescription, tag string) (oras.ReadOnlyTarget, error) {
	// 0. Create a file store
	fs, err := file.New("/tmp/")
	if err != nil {
		panic(err)
	}

	// 1. Add files to the file store
	fileDescriptors := make([]v1.Descriptor, 0, len(files))
	for name, f := range files {
		fPath := path.Join(rootDir, name)
		fp, err := os.Create(fPath)
		if err != nil {
			return nil, fmt.Errorf("failed to add file to storage: %v", err)
		}
		_, err = io.Copy(fp, f.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to copy to file in storage: %v", err)
		}
		if f.MediaType == "" {
			f.MediaType = "application/vnd.test.file"
		}
		fileDescriptor, err := fs.Add(ctx, name, f.MediaType, fPath)
		if err != nil {
			return nil, fmt.Errorf("failed to add file to storage: %v", err)
		}
		fileDescriptors = append(fileDescriptors, fileDescriptor)
		fmt.Printf("file descriptor for %s: %v\n", name, fileDescriptor)
	}

	// 2. Pack the files and tag the packed manifest
	artifactType := "application/vnd.test.artifact"
	opts := oras.PackManifestOptions{
		Layers: fileDescriptors,
	}
	manifestDescriptor, err := oras.PackManifest(ctx, fs, oras.PackManifestVersion1_1, artifactType, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to pack manifest: %v", err)
	}

	if err = fs.Tag(ctx, manifestDescriptor, tag); err != nil {
		return nil, fmt.Errorf("failed to tag manifest: %v", err)
	}
	return fs, nil
}
