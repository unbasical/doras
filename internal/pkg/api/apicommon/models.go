package apicommon

import (
	"errors"
	"fmt"
)

type SuccessResponse[T any] struct {
	Success T `json:"success"`
}

type ReadDeltaRequest struct {
	From               string   `json:"from"`
	To                 string   `json:"to"`
	AcceptedAlgorithms []string `json:"accepted_algorithms"`
}

type ReadDeltaResponse struct {
	TargetImage string `json:"target_image"`
	DeltaImage  string `json:"delta_image"`
}

// APIError wraps around the actual error for easier JSON parsing.
type APIError struct {
	InnerError APIErrorInner `json:"error"`
}

func (a APIError) Error() string {
	return a.InnerError.Error()
}

// APIErrorInner represents an error from the API.
type APIErrorInner struct {
	Code         int    `json:"code"`
	Message      string `json:"message,omitempty"`
	ErrorContext string `json:"context,omitempty"`
}

func (a APIErrorInner) Error() string {
	return fmt.Sprintf("server responded with error: status code: %v, message: %q, context: %q", a.Code, a.Message, a.ErrorContext)
}

// Is allows to call errors.Is() on APIError for better error handling.
func (a APIError) Is(target error) bool {
	var t APIError
	ok := errors.As(target, &t)
	if !ok {
		return false
	}
	// Compare the fields you care about.
	return a.InnerError.Message == t.InnerError.Message && a.InnerError.ErrorContext == t.InnerError.ErrorContext
}

// ErrImagesIdentical is returned by the API when the delta would have to be created from identical images.
var ErrImagesIdentical = APIError{InnerError: APIErrorInner{
	Message:      "artifacts are not compatible",
	ErrorContext: "from and to image are identical",
}}

// ErrImagesIncompatible is returned by the API when the delta would have to be created from identical images.
var ErrImagesIncompatible = APIError{InnerError: APIErrorInner{
	Message:      "artifacts are not compatible",
	ErrorContext: "cannot build a delta from images",
}}
