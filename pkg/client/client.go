package client

import "net/http"

type dorasBaseClient struct {
	serverURL string
	client    *http.Client
}

func newBaseClient(serverURL string) *dorasBaseClient {
	return &dorasBaseClient{
		serverURL: serverURL,
		client:    http.DefaultClient,
	}
}
