package client

type AuthTokenProvider interface {
	GetAuthToken() (string, error)
}
