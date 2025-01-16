package apicommon

import (
	"oras.land/oras-go/v2/registry/remote"
)

// Config is used to configure repository clients for Server.
type Config struct {
	RepoClients map[string]remote.Client
}
