package apicommon

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

type APIError struct {
	Error APIErrorInner `json:"error"`
}

type APIErrorInner struct {
	Code         int    `json:"code"`
	Message      string `json:"message,omitempty"`
	ErrorContext string `json:"context,omitempty"`
}
