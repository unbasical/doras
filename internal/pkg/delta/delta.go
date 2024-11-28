package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/unbasical/doras-server/internal/pkg/funcutils"

	tarDiff "github.com/containers/tar-diff/pkg/tar-diff"
	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
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

func createDelta(fromImage, toImage v1.Descriptor, fromReader, toReader io.ReadSeeker) (string, io.ReadCloser, error) {
	if fromImage.Annotations[ContentUnpack] != toImage.Annotations[ContentUnpack] {
		return "", nil, fmt.Errorf("mismatched contents, both need to be packed or not %v, %v", fromImage, toImage)
	}
	unpack := fromImage.Annotations[ContentUnpack] == "true"
	if unpack {
		// create tar diff
		optsTarDiff := tarDiff.NewOptions()
		pr, pw := io.Pipe()
		go func() {
			err := tarDiff.Diff(fromReader, toReader, pw, optsTarDiff)
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(err), true, "failed tardiff creation")
			funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")
		}()
		return "tardiff", pr, nil
	} else {
		// create bsdiff
		pr, pw := io.Pipe()
		go func() {
			err := bsdiff.Reader(fromReader, toReader, pw)
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(err), true, "failed bsdiff creation")
			funcutils.PanicOrLogOnErr(pw.Close, true, "failed to close pipe writer")

		}()
		return "bsdiff", pr, nil
	}
}

func CreateDelta(ctx context.Context, src oras.ReadOnlyTarget, dst oras.Target, fromImage, toImage v1.Descriptor) (*v1.Descriptor, error) {
	// TODO:
	// - handle case where delta exists already
	// - create dummy placeholder to communicate that the request is ongoing
	// - consider one of these two
	//   - use a local OCI layout for storage instead of writeBlobToTempfile
	//   - stream the data directly into the delta creation
	fromDescriptor, fromBlobReader, err := getBlobReaderForArtifact(ctx, src, fromImage)
	if err != nil {
		return nil, fmt.Errorf("failed to get from blob for from-image (%v): %v", fromImage, err)
	}
	defer funcutils.PanicOrLogOnErr(fromBlobReader.Close, true, "failed to close reader")

	toDescriptor, toBlobReader, err := getBlobReaderForArtifact(ctx, src, toImage)
	if err != nil {
		return nil, fmt.Errorf("failed to get to-blob for to-image (%v): %v", toImage, err)
	}
	defer funcutils.PanicOrLogOnErr(toBlobReader.Close, true, "failed to close reader")

	if fromDescriptor.Annotations[ContentUnpack] != toDescriptor.Annotations[ContentUnpack] {
		return nil, fmt.Errorf("mismatched contents, both need to be packed or not %v, %v", toDescriptor, toDescriptor)
	}
	tempDir := os.TempDir()
	// use go routine here
	fFrom, err := writeBlobToTempfile(tempDir, fromDescriptor, fromBlobReader)
	if err != nil {
		return nil, err
	}
	defer funcutils.PanicOrLogOnErr(fFrom.Close, false, "failed to close temp file")

	// use go routine here
	fTo, err := writeBlobToTempfile(tempDir, toDescriptor, toBlobReader)
	if err != nil {
		return nil, err
	}
	defer funcutils.PanicOrLogOnErr(fTo.Close, false, "failed to close temp file")

	algo, content, err := createDelta(*fromDescriptor, *toDescriptor, fFrom, fTo)
	if err != nil {
		return nil, err
	}
	fName := fmt.Sprintf("%s.patch.%s", deltaTag(fromImage, toImage), algo)
	fileNames := []string{fName}
	outputDir := tempDir
	outputPath := path.Join(outputDir, fName)

	fOut, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open delta output file (%v): %v", outputPath, err)
	}

	_, err = io.Copy(fOut, content)
	if err != nil {
		return nil, fmt.Errorf("failed to write to delta output file (%v): %v", outputPath, err)
	}

	// 0. Create a file store
	fs, err := file.New(outputDir)
	funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(err), true, "failed to create storage object")
	defer funcutils.PanicOrLogOnErr(fs.Close, false, "failed to close oras file storage")

	// 1. Add files to the file store
	mediaType := "application/octet-stream"
	fileDescriptors := make([]v1.Descriptor, 0, len(fileNames))
	for _, name := range fileNames {
		fileDescriptor, err := fs.Add(ctx, name, mediaType, outputPath)
		funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(err), true, "failed to add file to storage object")
		fromJson, errFrom := json.Marshal(fromImage)
		toJson, errTo := json.Marshal(toImage)
		err = funcutils.MultiError(errFrom, errTo)
		if err != nil {
			return nil, err
		}
		fileDescriptor.Annotations["from"] = string(fromJson)
		fileDescriptor.Annotations["to"] = string(toJson)
		fileDescriptors = append(fileDescriptors, fileDescriptor)
	}

	// 2. Pack the files and tag the packed manifest
	artifactType := "application/vnd.test.artifact"
	opts := oras.PackManifestOptions{
		Layers: fileDescriptors,
	}
	manifestDescriptor, err := oras.PackManifest(ctx, fs, oras.PackManifestVersion1_1, artifactType, opts)
	funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(err), true, "failed to pack manifest")
	fmt.Println("manifest descriptor:", manifestDescriptor)

	tag := deltaTag(fromImage, toImage)
	if err = fs.Tag(ctx, manifestDescriptor, tag); err != nil {
		return nil, fmt.Errorf("failed to tag delta manifest descriptor (%v): %v", manifestDescriptor, err)
	}
	descriptor, err := oras.Copy(ctx, fs, tag, dst, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to copy delta: %v", err)
	}
	return &descriptor, nil
}

func deltaTag(from v1.Descriptor, to v1.Descriptor) string {
	digestFormatFunc := func(target v1.Descriptor) string {
		return strings.ReplaceAll(target.Digest.String()[:32], ":", "-")
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
