package delta

import (
	"context"
	"encoding/json"
	"fmt"
	tar_diff "github.com/containers/tar-diff/pkg/tar-diff"
	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"io"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"os"
	"path"
	"strings"
)

const (
	BSDIFF = "bsdiff"
)
const ContentUnpack = "io.deis.oras.content.unpack"

func referenceFromDescriptor(d v1.Descriptor) string {
	return fmt.Sprintf("@%s", d.Digest.String())
}

func getBlobReaderForArtifact(ctx context.Context, src oras.ReadOnlyTarget, target v1.Descriptor) (*v1.Descriptor, io.ReadCloser, error) {
	r, err := src.Fetch(ctx, target)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch manifest: %v", err)
	}
	manifestData, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read manifest: %v", err)
	}
	var manifest v1.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal manifest: %v", err)
	}

	layers := manifest.Layers
	if len(layers) != 1 {
		return nil, nil, fmt.Errorf("expected exactly 1 layer, got %d", len(layers))
	}
	blobDescriptor := layers[0]
	blob, err := src.Fetch(ctx, blobDescriptor)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch blob: %v", err)
	}
	return &blobDescriptor, blob, nil
}

func panicOrLogOnErr(f func() error, panicOnErr bool, msg string) {
	err := f()
	if err != nil {
		if panicOnErr {
			panic(fmt.Sprintf("%s: %s", msg, err))
		} else {
			fmt.Printf("%s: %s\n", msg, err.Error())
		}
	}
}

func CreateDelta(ctx context.Context, src oras.ReadOnlyTarget, dst oras.Target, fromImage, toImage v1.Descriptor, alg string) (*v1.Descriptor, error) {
	fromDescriptor, fromBlobReader, err := getBlobReaderForArtifact(ctx, src, fromImage)
	if err != nil {
		return nil, fmt.Errorf("failed to get from blob for from-image (%v): %v", fromImage, err)
	}
	defer panicOrLogOnErr(fromBlobReader.Close, true, "failed to close reader")

	toDescriptor, toBlobReader, err := getBlobReaderForArtifact(ctx, src, toImage)
	if err != nil {
		return nil, fmt.Errorf("failed to get to-blob for to-image (%v): %v", toImage, err)
	}
	defer panicOrLogOnErr(toBlobReader.Close, true, "failed to close reader")

	if fromDescriptor.Annotations[ContentUnpack] != toDescriptor.Annotations[ContentUnpack] {
		return nil, fmt.Errorf("mismatched contents, both need to be packed or not %v, %v", toDescriptor, toDescriptor)
	}
	tempDir := os.TempDir()
	fFrom, err := writeBlobToTempfile(tempDir, fromDescriptor, fromBlobReader)
	if err != nil {
		return nil, err
	}
	fTo, err := writeBlobToTempfile(tempDir, toDescriptor, toBlobReader)
	if err != nil {
		return nil, err
	}
	var outputPath string
	var fileNames []string

	unpack := fromDescriptor.Annotations[ContentUnpack] == "true"
	if unpack {
		// create tar diff
		outputPath = path.Join(tempDir, "tardiff.patch")
		fOut, err := os.Create(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %v", err)
		}
		defer panicOrLogOnErr(fOut.Close, true, "failed to close output file")

		optsTarDiff := tar_diff.NewOptions()
		err = tar_diff.Diff(fFrom, fTo, fOut, optsTarDiff)
		if err != nil {
			return nil, err
		}
		fileNames = []string{"tardiff.patch"}
	} else {
		// create bsdiff
		outputPath = path.Join(tempDir, "bsdiff.patch")
		f, err := os.Create(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %v", err)
		}
		err = bsdiff.Reader(fromBlobReader, toBlobReader, f)
		if err != nil {
			return nil, err
		}
		fileNames = []string{"bsdiff.patch"}
	}

	// 0. Create a file store
	fs, err := file.New(os.TempDir())
	if err != nil {
		panic(err)
	}
	defer panicOrLogOnErr(fs.Close, true, "failed to close oras file storage")

	// 1. Add files to the file store
	mediaType := "application/octet-stream"
	fileDescriptors := make([]v1.Descriptor, 0, len(fileNames))
	for _, name := range fileNames {
		fileDescriptor, err := fs.Add(ctx, name, mediaType, outputPath)
		if err != nil {
			panic(err)
		}
		fileDescriptors = append(fileDescriptors, fileDescriptor)
	}

	// 2. Pack the files and tag the packed manifest
	artifactType := "application/vnd.test.artifact"
	opts := oras.PackManifestOptions{
		Layers: fileDescriptors,
	}
	manifestDescriptor, err := oras.PackManifest(ctx, fs, oras.PackManifestVersion1_1, artifactType, opts)
	if err != nil {
		panic(err)
	}
	fmt.Println("manifest descriptor:", manifestDescriptor)

	tag := deltaTag(fromImage, toImage)
	if err = fs.Tag(ctx, manifestDescriptor, tag); err != nil {
		panic(err)
	}
	descriptor, err := oras.Copy(ctx, fs, tag, dst, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, err
	}
	return &descriptor, nil
}

func deltaTag(from v1.Descriptor, to v1.Descriptor) string {
	digestFormatFunc := func(target v1.Descriptor) string {
		return strings.ReplaceAll(target.Digest.String(), ":", "-")
	}
	return fmt.Sprintf("%s_%s", digestFormatFunc(from), digestFormatFunc(to))
}

func writeBlobToTempfile(outdir string, target *v1.Descriptor, content io.Reader) (*os.File, error) {
	f, err := os.Create(path.Join(outdir, strings.ReplaceAll(target.Digest.Encoded(), ":", "-")))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file to store blob: %v", err)
	}
	written, err := io.Copy(f, content)
	if err != nil || written != target.Size {
		return nil, fmt.Errorf("failed to copy blob to temp file or did not get enough bytes (got=%d, expected=%d): %v", err, target.Size, written)
	}
	// seek to start
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Artifact describes an artifact manifest.
// This structure provides `application/vnd.oci.artifact.manifest.v1+json` mediatype when marshalled to JSON.
//
// This manifest type was introduced in image-spec v1.1.0-rc1 and was removed in
// image-spec v1.1.0-rc3. It is not part of the current image-spec and is kept
// here for Go compatibility.
//
// Reference: https://github.com/opencontainers/image-spec/pull/999
type Artifact struct {
	// MediaType is the media type of the object this schema refers to.
	MediaType string `json:"mediaType"`

	// ArtifactType is the IANA media type of the artifact this schema refers to.
	ArtifactType string `json:"artifactType"`

	// Blobs is a collection of blobs referenced by this manifest.
	Blobs []v1.Descriptor `json:"blobs,omitempty"`

	// Subject (reference) is an optional link from the artifact to another manifest forming an association between the artifact and the other manifest.
	Subject *v1.Descriptor `json:"subject,omitempty"`

	// Annotations contains arbitrary metadata for the artifact manifest.
	Annotations map[string]string `json:"annotations,omitempty"`
}
