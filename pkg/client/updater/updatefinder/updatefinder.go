package updatefinder

type UpdateFinder interface {
	// LoadUpdate
	// updateImage is an image with the digest
	LoadUpdate(image string, isInitialized bool) (isDelta bool, updateImage string, err error)
}
