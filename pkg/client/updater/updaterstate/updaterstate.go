package updaterstate

import (
	"fmt"
	"github.com/opencontainers/go-digest"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"strings"
)

// State represents the state of a doras updater.
type State struct {
	Version        string                   `json:"version"`
	ArtifactStates map[string]ArtifactState `json:"artifact_states"`
}

// ArtifactState represents the state of an image stored in a directory.
type ArtifactState struct {
	ImageDigest     digest.Digest `json:"image_digest"`
	DirectoryDigest digest.Digest `json:"directory_digest"`
}

// SetArtifactState update the internal state so it knows which version of an image we have in a directory.
func (s *State) SetArtifactState(artifactDirectory, image string, dirHash digest.Digest) error {
	repoName, dig, isDigest, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return err
	}
	if !isDigest {
		return fmt.Errorf("image %s is not an image with digest", image)
	}
	k := fmt.Sprintf("(%s,%s)", artifactDirectory, repoName)
	parse, err := digest.Parse(strings.TrimPrefix(dig, "@"))
	if err != nil {
		return err
	}
	artifactState := ArtifactState{
		ImageDigest:     parse,
		DirectoryDigest: dirHash,
	}
	s.ArtifactStates[k] = artifactState
	return nil
}

// GetArtifactState returns the digest which is currently rolled out for a specific version.
func (s *State) GetArtifactState(artifactDirectory, image string) (ArtifactState, error) {
	k := fmt.Sprintf("(%s,%s)", artifactDirectory, image)
	log.Debugf("looking for version with key:%q", k)
	artifactState, ok := s.ArtifactStates[k]
	if !ok {
		return ArtifactState{}, fmt.Errorf("artifact state not found for %s", image)
	}
	return artifactState, nil
}
