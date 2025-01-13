package apidelegate

type APIDelegate interface {
	ExtractParams() (fromImage, toImage string, acceptedAlgorithms []string, err error)
	ExtractClientToken() (string, error)
	HandleError(err error, msg string)
	HandleSuccess(response any)
	HandleAccepted()
}
