package updater

import (
	"fmt"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/pkg/client/updater/fetcher"
	"github.com/unbasical/doras/pkg/client/updater/statemanager"
	"github.com/unbasical/doras/pkg/client/updater/updaterstate"
	"os"
	"strings"

	"github.com/unbasical/doras/internal/pkg/algorithmchoice"
	"github.com/unbasical/doras/internal/pkg/compression/gzip"
	"github.com/unbasical/doras/internal/pkg/compression/zstd"
	"github.com/unbasical/doras/internal/pkg/delta/bsdiff"
	"github.com/unbasical/doras/internal/pkg/delta/tardiff"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras/pkg/client/edgeapi"
)

// Client is used to run delta updates in Doras.
type Client struct {
	opts       clientOpts
	edgeClient *edgeapi.Client
	reg        fetcher.RegistryDelegate
	state      *statemanager.Manager[updaterstate.State]
}

// Pull an image from the registry.
func (c *Client) Pull(image string) error {
	for {
		exists, err := c.PullAsync(image)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
	}
}

func getPatcherChoice(d *v1.Descriptor) (algorithmchoice.PatcherChoice, error) {
	choice := algorithmchoice.PatcherChoice{}
	trimmed := strings.TrimPrefix(d.MediaType, "application/")
	split := strings.Split(trimmed, "+")
	if len(split) == 2 {
		switch split[1] {
		case "gzip":
			choice.Decompressor = gzip.NewDecompressor()
		case "zstd":
			choice.Decompressor = zstd.NewDecompressor()
		default:
			panic("not supported")
		}
	}
	switch split[0] {
	case "bsdiff":
		choice.Patcher = bsdiff.NewPatcher()
	case "tardiff":
		choice.Patcher = tardiff.NewPatcher()
	default:
		return algorithmchoice.PatcherChoice{}, fmt.Errorf("unsupported patcher: %s", split[0])
	}
	return choice, nil
}

// PullAsync Pull delta, but do not block if the delta has not been created yet.
// The result of the pull is according to the client configuration.
func (c *Client) PullAsync(target string) (exists bool, err error) {
	s, err := c.state.Load()
	if err != nil {
		return false, err
	}
	repoName, _, _, err := ociutils.ParseOciImageString(target)
	d, err := s.GetArtifactState(c.opts.OutputDirectory, repoName)
	if err != nil {
		panic("loading full artifacts is not yet implemented")
	}
	currentImage := fmt.Sprintf("%s@%s", repoName, d.Encoded())
	if currentImage == target {
		return true, nil
	}
	res, exists, err := c.edgeClient.ReadDeltaAsync(currentImage, target, c.opts.AcceptedAlgorithms)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	deltaDir, err := os.MkdirTemp(c.opts.InternalDirectory, "deltas-*")
	if err != nil {
		return false, err
	}
	_, deltas, err := c.reg.ResolveAndLoadToPath(res.DeltaImage, deltaDir)
	if err != nil {
		return false, err
	}
	for _, d := range deltas {
		p, err := getPatcherChoice(&d.D)
		if err != nil {
			return false, err
		}
		if p.Patcher.Name() == "bsdiff" {
			panic("not yet implemented")
		}
		fp, err := os.Open(d.Path)
		if err != nil {
			return false, err
		}
		decompressedPatch, err := p.Decompress(fp)
		if err != nil {
			return false, err
		}
		// digest is not provided, however we already verified a digest while fetching the deltas
		err = p.PatchFilesystem(c.opts.OutputDirectory, decompressedPatch, nil)
		if err != nil {
			return false, err
		}
	}
	err = c.state.ModifyState(func(u *updaterstate.State) error {
		return u.SetArtifactState(c.opts.OutputDirectory, res.TargetImage)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
