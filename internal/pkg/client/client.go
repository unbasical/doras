package client

import "net/http"

// DorasBaseClient serves as the base for clients that interact with Doras servers.
type DorasBaseClient struct {
	DorasURL      string
	Client        *http.Client
	TokenProvider AuthProvider
}

// NewBaseClient constructs a DorasBaseClient.
func NewBaseClient(serverURL string, tokenProvider AuthProvider) *DorasBaseClient {
	return &DorasBaseClient{
		DorasURL:      serverURL,
		Client:        http.DefaultClient,
		TokenProvider: tokenProvider,
	}
}
