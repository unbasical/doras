package apidelegate

type APIDelegate interface {
	ExtractParams() (fromImage, toImage string, acceptedAlgorithms []string, err error)
	HandleError(err error, msg string)
	HandleSuccess(response any)
	HandleAccepted()
}
