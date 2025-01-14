package client

import "net/http"

type DorasBaseClient struct {
	DorasURL      string
	Client        *http.Client
	TokenProvider AuthTokenProvider
}

func NewBaseClient(serverURL string, tokenProvider AuthTokenProvider) *DorasBaseClient {
	return &DorasBaseClient{
		DorasURL:      serverURL,
		Client:        http.DefaultClient,
		TokenProvider: tokenProvider,
	}
}
