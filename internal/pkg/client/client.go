package client

import (
	"net/http"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// DorasBaseClient serves as the base for clients that interact with Doras servers.
type DorasBaseClient struct {
	DorasURL string
	Client   *http.Client
	auth.CredentialFunc
}

// NewBaseClient constructs a DorasBaseClient.
func NewBaseClient(serverURL string, credentialFunc auth.CredentialFunc) *DorasBaseClient {
	return &DorasBaseClient{
		DorasURL:       serverURL,
		Client:         http.DefaultClient,
		CredentialFunc: credentialFunc,
	}
}
