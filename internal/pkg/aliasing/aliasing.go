package aliasing

type Aliasing interface {
	// AddAlias
	// The implementation has to handle the sanitization of the identifier.
	AddAlias(alias string, target string) error
	// ResolveAlias
	// The implementation has to handle the sanitization of the identifier.
	ResolveAlias(alias string) (string, error)
}
