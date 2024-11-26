package error

import "errors"

type APIError struct {
	Error APIErrorInner `json:"error"`
}

type APIErrorInner struct {
	Code    error  `json:"code"`
	Message string `json:"message,omitempty"`
}

//nolint:golint,gochecknoglobals // errors.New() is not const
var (
	ErrAliasNotFound               = errors.New("alias not found")
	ErrDeltaNotFound               = errors.New("delta not found")
	ErrArtifactNotFound            = errors.New("artifact not found")
	ErrAliasExists                 = errors.New("alias exists")
	ErrArtifactNotProvided         = errors.New("artifact not provided")
	ErrInternal                    = errors.New("internal")
	ErrMissingRequestBody          = errors.New("missing request body")
	ErrUnsupportedDiffingAlgorithm = errors.New("unsupported diffing algorithm")
)
