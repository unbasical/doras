package error

import "errors"

type CloudAPIError struct {
	Error CloudAPIErrorInner `json:"error"`
}

type CloudAPIErrorInner struct {
	Code    error  `json:"code"`
	Message string `json:"message,omitempty"`
}

//nolint:golint,gochecknoglobals // errors.New() is not const
var (
	ErrAliasNotFound               = errors.New("AliasNotFound")
	ErrDeltaNotFound               = errors.New("DeltaNotFound")
	ErrArtifactNotFound            = errors.New("ArtifactNotFound")
	ErrAliasExists                 = errors.New("AliasExists")
	ErrArtifactNotProvided         = errors.New("ArtifactNotProvided")
	ErrInternal                    = errors.New("Internal")
	ErrMissingRequestBody          = errors.New("MissingRequestBody")
	ErrUnsupportedDiffingAlgorithm = errors.New("UnsupportedDiffingAlgorithm")
)
