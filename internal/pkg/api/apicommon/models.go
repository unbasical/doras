package apicommon

import v1 "github.com/opencontainers/image-spec/specs-go/v1"

type SuccessResponse[T any] struct {
	Success T `json:"success"`
}

type CreateOCIArtifactRequest struct {
	Image string `json:"image"`
}

type ReadDeltaRequest struct {
	From               string   `json:"from"`
	To                 string   `json:"to"`
	AcceptedAlgorithms []string `json:"accepted_algorithms"`
}

type ReadDeltaResponse struct {
	Desc v1.Descriptor `json:"descriptor"`
}

type APIError struct {
	Error APIErrorInner `json:"error"`
}

type APIErrorInner struct {
	Code         int    `json:"code"`
	Message      string `json:"message,omitempty"`
	ErrorContext string `json:"context,omitempty"`
}
