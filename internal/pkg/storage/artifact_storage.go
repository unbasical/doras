package storage

import (
	"github.com/unbasical/doras-server/internal/pkg/artifact"
	"github.com/unbasical/doras-server/internal/pkg/delta"
)

// ArtifactStorage is an interface that abstracts loading and storing artifacts.
type ArtifactStorage interface {
	// LoadArtifact
	//The implementation has to handle the sanitization of the identifier.
	LoadArtifact(identifier string) (artifact.Artifact, error)
	// StoreArtifact
	//The implementation has to handle the sanitization of the identifier.
	StoreArtifact(artifact artifact.Artifact, identifier string) error
	// StoreDelta stores the delta with the provided identifier.
	//The implementation has to handle the sanitization of the identifier.
	StoreDelta(d delta.ArtifactDelta, identifier string) error
	// LoadDelta
	//The implementation has to handle the sanitization of the identifier.
	LoadDelta(identifier string) (delta.ArtifactDelta, error)
}
