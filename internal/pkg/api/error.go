package api

type apiError struct {
	Error apiErrorInner `json:"error"`
}

type apiErrorInner struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

const ArtifactExists = "ArtifactExists"
