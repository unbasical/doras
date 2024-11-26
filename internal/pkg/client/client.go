package client

import "net/http"

type DorasBaseClient struct {
	DorasURL string
	Client   *http.Client
}

func NewBaseClient(serverURL string) *DorasBaseClient {
	return &DorasBaseClient{
		DorasURL: serverURL,
		Client:   http.DefaultClient,
	}
}
