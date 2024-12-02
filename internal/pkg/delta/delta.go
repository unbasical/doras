package delta

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"io"
	"os"
	"path"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/unbasical/doras-server/pkg/constants"

	tarDiff "github.com/containers/tar-diff/pkg/tar-diff"
	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
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
	tag := deltaTag(fromImage, toImage)

	existingDescriptor, err := oras.Resolve(ctx, dst, tag, oras.DefaultResolveOptions)
	if err == nil {
		return &existingDescriptor, nil
	}
	fromDigest := "sha256:" + fromImage.Digest.Encoded()
	toDigest := "sha256:" + toImage.Digest.Hex()

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
	defer funcutils.PanicOrLogOnErr(content.Close, false, "failed to close delta reader")
	fName := fmt.Sprintf("%s.patch.%s", deltaTag(fromImage, toImage), algo)
	outputPath := path.Join(tempDir, fName)

	fOut, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open delta output file (%v): %v", outputPath, err)
	}

	hasher := sha256.New()
	contentReader := io.TeeReader(content, hasher)
	_, err = io.Copy(fOut, contentReader)
	if err != nil {
		return nil, fmt.Errorf("failed to write to delta output file (%v): %v", outputPath, err)
	}
	stat, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat delta output file (%v): %v", outputPath, err)
	}
	deltaDigest := digest.NewDigest("sha256", hasher)
	d := v1.Descriptor{
		MediaType: "application/vnd.oci.image.layer.v1.tar",
		Digest:    deltaDigest,
		Size:      stat.Size(),
		Annotations: map[string]string{
			"org.opencontainers.image.title": fName,
		},
		ArtifactType: "application/vnd.test.artifact",
	}
	f, err := os.Open(fOut.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to open delta output file (%v): %v", outputPath, err)
	}
	err = dst.Push(ctx, d, f)
	if err != nil {
		return nil, fmt.Errorf("failed to push delta output file (%v): %v", outputPath, err)
	}

	opts := oras.PackManifestOptions{
		Layers: []v1.Descriptor{d},
		ManifestAnnotations: map[string]string{
			constants.DorasAnnotationFrom:      fromDigest,
			constants.DorasAnnotationTo:        toDigest,
			constants.DorasAnnotationAlgorithm: algo,
		},
	}
	artifactType := "application/vnd.test.artifact"
	manifestDescriptor, err := oras.PackManifest(ctx, dst, oras.PackManifestVersion1_1, artifactType, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to pack delta manifest (%v): %v", outputPath, err)
	}

	if err = dst.Tag(ctx, manifestDescriptor, tag); err != nil {
		return nil, fmt.Errorf("failed to tag delta manifest descriptor (%v): %v", manifestDescriptor, err)
	}
	// Annotations are not reliably stored in the descriptor, delete them so no one relies on them.
	manifestDescriptor.Annotations = nil
	return &manifestDescriptor, nil
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
