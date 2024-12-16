package apicommon

import v1 "github.com/opencontainers/image-spec/specs-go/v1"

type SuccessResponse[T any] struct {
	Success T `json:"success"`
}

type ReadDeltaRequest struct {
	From               string   `json:"from"`
	To                 string   `json:"to"`
	AcceptedAlgorithms []string `json:"accepted_algorithms"`
}

type ReadDeltaResponse struct {
	TargetImage     string        `json:"target_image"`
	DeltaDescriptor v1.Descriptor `json:"delta_descriptor"`
}

type APIError struct {
	Error APIErrorInner `json:"error"`
}

type APIErrorInner struct {
	Code         int    `json:"code"`
	Message      string `json:"message,omitempty"`
	ErrorContext string `json:"context,omitempty"`
}
