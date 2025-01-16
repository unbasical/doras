package apidelegate

import (
	auth2 "github.com/unbasical/doras-server/internal/pkg/auth"
)

type APIDelegate interface {
	ExtractParams() (fromImage, toImage string, acceptedAlgorithms []string, err error)
	ExtractClientAuth() (auth2.RegistryAuth, error)
	HandleError(err error, msg string)
	HandleSuccess(response any)
	HandleAccepted()
}
