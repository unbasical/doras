package error

import (
	"errors"
)

//nolint:golint,gochecknoglobals // errors.New() is not const
var (
	ErrAliasNotFound               = errors.New("alias not found")
	ErrDeltaNotFound               = errors.New("delta not found")
	ErrArtifactNotFound            = errors.New("artifact not found")
	ErrArtifactNotProvided         = errors.New("artifact not provided")
	ErrInternal                    = errors.New("internal")
	ErrMissingRequestBody          = errors.New("missing request body")
	ErrUnsupportedDiffingAlgorithm = errors.New("unsupported diffing algorithm")
	ErrUnmarshal                   = errors.New("failed to unmarshall request body")
	ErrNotYetImplemented           = errors.New("not yet implemented")
	ErrBadRequest                  = errors.New("bad request")
	ErrInvalidOciImage             = errors.New("invalid oci image")
	ErrMissingQueryParam           = errors.New("missing query param")
)
