package updaterstate

import (
	"fmt"
	"github.com/opencontainers/go-digest"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"strings"
)

type State struct {
	Version        string            `json:"version"`
	ArtifactStates map[string]string `json:"artifact_states"`
}

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

func (s *State) GetArtifactState(artifactDirectory, image string) (*digest.Digest, error) {
	k := fmt.Sprintf("(%s,%s)", artifactDirectory, image)
	dig, ok := s.ArtifactStates[k]
	if !ok {
		return nil, fmt.Errorf("artifact state not found for %s", image)
	}
	d := digest.NewDigestFromEncoded("sha256", dig)
	return &d, nil
}
