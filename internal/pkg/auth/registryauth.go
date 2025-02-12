package auth

import auth2 "oras.land/oras-go/v2/registry/remote/auth"

// RegistryAuth abstracts over credentials that can be used to access registries.
type RegistryAuth interface {
	// CredentialFunc returns an authentication mechanism that can be used to access the registry at the provided URL.
	CredentialFunc(scope string) (auth2.CredentialFunc, error)
}

type registryAuthToken struct {
	credential auth2.Credential
}

func (c *registryAuthToken) CredentialFunc(scope string) (auth2.CredentialFunc, error) {
	return auth2.StaticCredential(scope, c.credential), nil
}

func NewClientAuthFromToken(token string) RegistryAuth {
	return &registryAuthToken{credential: auth2.Credential{AccessToken: token}}
}

func NewClientAuthFromUsernamePassword(username, password string) RegistryAuth {
	return &registryAuthToken{credential: auth2.Credential{Username: username, Password: password}}
}
