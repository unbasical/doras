package orastesttarget

import (
	"bytes"
	"context"
	"fmt"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"io"
)

type OCIArtifact struct {
	ident ocispec.Descriptor
	data  []byte
}

type DummyTarget struct {
	artifacts map[string]OCIArtifact
	tags      map[string]ocispec.Descriptor
}

type NopCloser struct {
}

func (n NopCloser) Close() error {
	return nil
}

func (e *DummyTarget) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	a, ok := e.artifacts[target.Digest.Hex()]
	if !ok {
		return nil, fmt.Errorf("artifact not found: %s", target.Digest.Hex())
	}
	return struct {
		io.Reader
		io.Closer
	}{
		Reader: bytes.NewReader(a.data),
		Closer: NopCloser{},
	}, nil
}

func (e *DummyTarget) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	_, err := e.Resolve(ctx, target.Digest.Hex())
	if err != nil {
		return false, err
	}
	return true, nil
}

func (e *DummyTarget) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	a, ok := e.artifacts[reference]
	if !ok {
		d, ok := e.tags[reference]
		if !ok {
			return ocispec.Descriptor{}, fmt.Errorf("reference %s not found", reference)
		}
		a, ok = e.artifacts[d.Digest.Hex()]
		if !ok {
			return ocispec.Descriptor{}, fmt.Errorf("reference %s not found", reference)
		}
		return a.ident, nil
	}
	return a.ident, nil
}
