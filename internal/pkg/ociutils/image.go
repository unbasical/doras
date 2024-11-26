package ociutils

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"net/url"
	"strings"

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
