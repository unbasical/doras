package ociutils

import (
	"fmt"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type ArtifactFetcher interface {
	FetchDelta(currentVersion, image, outDir string) (algo, fPath string, err error)
	FetchArtifact(image, outDir string) (fPath string, err error)
}

type fetcher struct {
	auth.CredentialFunc
}

func (f *fetcher) FetchDelta(currentVersion, image, outDir string) (algo, fPath string, err error) {
	//TODO implement me
	name, _, isDigest, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return "", "", err
	}
	if !isDigest {
		return "", "", fmt.Errorf("image %s is identifeid by a digest", image)
	}
	_, err = remote.NewRepository(name)
	if err != nil {
		return "", "", err
	}
	// use ingest here
	panic("todo")
}

func (f *fetcher) FetchArtifact(image, outDir string) (fPath string, err error) {
	//TODO implement me
	panic("implement me")
}
