package client

import (
	"context"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"net/url"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type AuthProvider interface {
	GetAuth() (auth.Credential, error)
}

type credentialFuncTokenProvider struct {
	auth.CredentialFunc
	registry url.URL
}

// NewCredentialFuncTokenProvider creates a token provider that uses the provided auth.CredentialFunc to load access tokens.
// Only works if the credential function is token based.
func NewCredentialFuncTokenProvider(creds auth.CredentialFunc, image string) (AuthProvider, error) {
	serverUrl, err := ociutils.ParseOciUrl(image)
	if err != nil {
		return nil, err
	}
	return &credentialFuncTokenProvider{
		CredentialFunc: creds,
		registry:       *serverUrl,
	}, nil
}

func (c *credentialFuncTokenProvider) GetAuth() (auth.Credential, error) {
	serverUrl := c.registry.Host
	credential, err := c.CredentialFunc(context.Background(), serverUrl)
	if err != nil {
		return auth.Credential{}, err
	}
	return credential, nil
}
