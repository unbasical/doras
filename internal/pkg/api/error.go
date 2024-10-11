package api

type cloudAPIError struct {
	Error cloudAPIErrorInner `json:"error"`
}

type cloudAPIErrorInner struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

const (
	DorasAliasNotFoundError       = "AliasNotFound"
	DorasArtifactNotFoundError    = "ArtifactNotFound"
	DorasAliasExistsError         = "AliasExists"
	DorasArtifactNotProvidedError = "ArtifactNotProvided"
	DorasInternalError            = "Internal"
)
