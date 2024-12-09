package engine

import (
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/pkg/delta"
	"github.com/unbasical/doras-server/pkg/storage"
)

type Engine interface {
	storage.Engine
	delta.Patcher
	delta.Differ
}

type ServerEngine interface {
	DiffAndStore(from, to v1.Descriptor, algorithm string)
}
