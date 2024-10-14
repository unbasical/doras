package api

import "errors"

type cloudAPIError struct {
	Error cloudAPIErrorInner `json:"error"`
}

type cloudAPIErrorInner struct {
	Code    error  `json:"code"`
	Message string `json:"message,omitempty"`
}

var (
	DorasAliasNotFoundError               = errors.New("AliasNotFound")
	DorasDeltaNotFoundError               = errors.New("DeltaNotFound")
	DorasArtifactNotFoundError            = errors.New("ArtifactNotFound")
	DorasAliasExistsError                 = errors.New("AliasExists")
	DorasArtifactNotProvidedError         = errors.New("ArtifactNotProvided")
	DorasInternalError                    = errors.New("Internal")
	DorasMissingRequestBodyError          = errors.New("MissingRequestBody")
	DorasUnsupportedDiffingAlgorithmError = errors.New("UnsupportedDiffingAlgorithm")
)
