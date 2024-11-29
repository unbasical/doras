package ociutils

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/funcutils"
	"io"
	"net/url"
	"oras.land/oras-go/v2"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/samber/lo"
)

type ImageIdentifier struct {
	registryURL string
	repository  string
	tag         string
	digest      string
}

func (i ImageIdentifier) Digest() (string, error) {
	if i.digest != "" {
		return "", errors.New("image has no digest")
	}
	return i.digest, nil
}

func (i ImageIdentifier) Tag() (string, error) {
	if i.tag == "" {
		return "", errors.New("image has no tag")
	}
	return i.tag, nil
}

func (i ImageIdentifier) Repository() string {
	return i.repository
}

func (i ImageIdentifier) RegistryURL() string {
	return i.registryURL
}

func (i ImageIdentifier) TagOrDigest() string {
	if i.tag != "" {
		return i.tag
	}
	if i.digest != "" {
		return i.digest
	}
	panic(fmt.Sprintf("misconstructed image identifier: %s", i))
}

func NewImageIdentifier(image string) (*ImageIdentifier, error) {
	if !strings.HasPrefix(image, "oci://") {
		image = "oci://" + image
	}
	imageURL, err := url.Parse(image)
	if err != nil {
		return nil, err
	}
	if !(imageURL.Scheme == "docker" || imageURL.Scheme == "oci") {
		return nil, fmt.Errorf("invalid image URL (unsupported scheme): %s", image)
	}

	split := strings.Split(imageURL.Path, "/")
	if len(split) == 0 {
		return nil, fmt.Errorf("invalid image URL (empty path): %s", image)
	}
	split = lo.Filter(split, func(item string, _ int) bool { return item != "" })

	splitDigest := strings.Split(split[len(split)-1], "@sha256:")
	if len(splitDigest) == 2 {
		repository := strings.Join(lo.Interleave(split[:len(split)-1], splitDigest[:1]), "/")
		return &ImageIdentifier{
			registryURL: imageURL.Host,
			repository:  repository,
			digest:      fmt.Sprintf("sha256:%s", splitDigest[1]),
		}, nil
	}

	splitTag := strings.Split(split[len(split)-1], ":")
	if len(splitTag) == 2 {
		repository := strings.Join(lo.Interleave(split[:len(split)-1], splitTag[:1]), "/")
		return &ImageIdentifier{
			registryURL: imageURL.Host,
			repository:  repository,
			tag:         splitTag[1],
		}, nil
	}
	return nil, fmt.Errorf("invalid image URL (empty digest/tag): %s", image)
}

func GetLayers(ctx context.Context, src oras.ReadOnlyTarget, rootDescriptor v1.Descriptor) ([]v1.Descriptor, error) {
	r, err := src.Fetch(ctx, rootDescriptor)
	if err != nil {
		return nil, err
	}
	defer funcutils.PanicOrLogOnErr(r.Close, false, "failed to close fetch reader")
	mf, err := ParseManifestJSON(r)
	if err != nil {
		return nil, err
	}
	return mf.Layers, nil
}

func GetBlobDescriptor(ctx context.Context, src oras.ReadOnlyTarget, rootDescriptor v1.Descriptor) (*v1.Descriptor, error) {
	layers, err := GetLayers(ctx, src, rootDescriptor)
	if err != nil {
		return nil, err
	}
	if len(layers) != 1 {
		return nil, fmt.Errorf("unexpected amount of layer (!= 1): %v", layers)
	}
	return &layers[0], nil
}

func ParseManifestJSON(data io.Reader) (*v1.Manifest, error) {
	var manifest *v1.Manifest
	err := json.NewDecoder(data).Decode(&manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func GetDescriptor(data []byte) v1.Descriptor {
	hasher := sha256.New()
	hasher.Write(data)
	descriptor := v1.Descriptor{
		MediaType:    "", // TODO: set media type
		Digest:       digest.NewDigest("sha256", hasher),
		Size:         int64(len(data)),
		URLs:         nil,
		Annotations:  nil, // TODO: add artifact name
		Data:         nil,
		Platform:     nil,
		ArtifactType: "", // TODO: set artifact type
	}
	return descriptor
}

var reDigest = regexp.MustCompile(`\S*@sha256:[a-f0-9]{64}$`)

func IsDigest(imageOrTag string) bool {
	return reDigest.MatchString(imageOrTag)
}
