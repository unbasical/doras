package storage

import "oras.land/oras-go/v2"

type ReadOnlyEngine interface {
	oras.ReadOnlyTarget
}

type Engine interface {
	oras.Target
}
