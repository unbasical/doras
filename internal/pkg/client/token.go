package client

import "oras.land/oras-go/v2/registry/remote/auth"

type AuthProvider interface {
	GetAuth() (auth.Credential, error)
}
