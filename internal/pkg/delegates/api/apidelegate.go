package apidelegate

import "oras.land/oras-go/v2/registry/remote/auth"

type APIDelegate interface {
	ExtractParams() (fromImage, toImage string, acceptedAlgorithms []string, err error)
	ExtractClientAuth() (ClientAuth, error)
	HandleError(err error, msg string)
	HandleSuccess(response any)
	HandleAccepted()
}

type ClientAuth interface {
	CredentialFunc(scope string) (auth.CredentialFunc, error)
}

type clientAuthToken struct {
	token string
}

func (c *clientAuthToken) CredentialFunc(scope string) (auth.CredentialFunc, error) {
	return auth.StaticCredential(scope, auth.Credential{
		AccessToken: c.token,
	}), nil
}

func NewClientAuthFromToken(token string) ClientAuth {
	return &clientAuthToken{token: token}
}
