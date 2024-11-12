package apicommon

type SuccessResponse[T any] struct {
	Success T `json:"success"`
}
