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
	Version        string            `json:"version"`
	ArtifactStates map[string]string `json:"artifact_states"`
}

// SetArtifactState update the internal state so it knows which version of an image we have in a directory.
func (s *State) SetArtifactState(artifactDirectory, image string) error {
	repoName, dig, isDigest, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return err
	}
	if !isDigest {
		return fmt.Errorf("image %s is not an image with digest", image)
	}
	k := fmt.Sprintf("(%s,%s)", artifactDirectory, repoName)
	s.ArtifactStates[k] = strings.TrimPrefix(dig, "@sha256:")
	return nil
}

// GetArtifactState returns the digest which is currently rolled out for a specific version.
func (s *State) GetArtifactState(artifactDirectory, image string) (*digest.Digest, error) {
	k := fmt.Sprintf("(%s,%s)", artifactDirectory, image)
	log.Debugf("looking for version with key:%q", k)
	dig, ok := s.ArtifactStates[k]
	if !ok {
		return nil, fmt.Errorf("artifact state not found for %s", image)
	}
	d := digest.NewDigestFromEncoded("sha256", dig)
	return &d, nil
}
