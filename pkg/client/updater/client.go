package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"golang.org/x/mod/sumdb/dirhash"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	"github.com/unbasical/doras/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/internal/pkg/utils/tarutils"
	"github.com/unbasical/doras/pkg/backoff"
	"github.com/unbasical/doras/pkg/client/updater/fetcher"
	"github.com/unbasical/doras/pkg/client/updater/statemanager"
	"github.com/unbasical/doras/pkg/client/updater/updaterstate"
	"github.com/unbasical/doras/pkg/constants"

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
	opts          clientOpts
	edgeClient    edgeapi.DeltaApiClient
	reg           fetcher.ArtifactLoader
	state         *statemanager.Manager[updaterstate.State]
	ctx           context.Context
	backoff       backoff.Strategy
	patcherTmpDir string
}

// Pull an image from the registry.
// This is just a wrapper around PullAsync that blocks until it succeeds or errors.
func (c *Client) Pull(image string) error {
	//TODO: add backoff mechanism from API client
	for {
		exists, err := c.PullAsync(image)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		err = c.backoff.Wait()
		if err != nil {
			return err
		}
	}
}

// getPatcherChoice extracts the algorithms that were used to create the delta from the provided v1.Descriptor.
func (c *Client) getPatcherChoice(d *v1.Descriptor, patcherTmpDir string) (algorithmchoice.PatcherChoice, error) {
	choice := algorithmchoice.PatcherChoice{
		// initialize with default
		Decompressor: compressionutils.NewNopDecompressor(),
	}
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
		choice.Patcher = bsdiff.NewPatcherWithTempDir(patcherTmpDir)
	case "tardiff":
		choice.Patcher = tardiff.NewPatcherWithTempDir(patcherTmpDir, c.opts.KeepOldDir, c.opts.OutputDirPermissions)
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
	if err != nil {
		return false, err
	}
	// find out what the current version is, if there is none load a full image
	d, err := s.GetArtifactState(c.opts.OutputDirectory, repoName)
	if err != nil {
		log.WithError(err).Debugf("got err:%q while loading state, attempting to load full image", err)
		return c.pullFullImage(target)
	}
	outputDirectoryHash, err := dirhash.HashDir(c.opts.OutputDirectory, "", dirhash.Hash1)
	if err != nil {
		return false, err
	}
	if d.DirectoryDigest != digest.Digest(outputDirectoryHash) {
		log.WithError(err).Warn("detected modifications to output directory, doing a clean pull")
		return c.pullFullImage(target)
	}
	// if we have an initial state we want to use a delta update
	return c.pullDeltaImageAsync(target, repoName, &d.ImageDigest)
}

func (c *Client) pullDeltaImageAsync(target string, repoName string, currentVersion *digest.Digest) (bool, error) {
	currentImage := fmt.Sprintf("%s@%s", repoName, currentVersion.String())
	// request delta from server asynchronously
	res, exists, err := c.edgeClient.ReadDeltaAsync(currentImage, target, c.opts.AcceptedAlgorithms)
	if err != nil {
		if errors.Is(err, apicommon.ErrImagesIdentical) {
			log.Info("already up-to-date")
			return true, nil
		}
		if errors.Is(err, apicommon.ErrImagesIncompatible) {
			return c.pullFullImage(target)
		}
		return false, err
	}
	if !exists {
		return false, nil
	}
	log.Info("attempting delta update")
	deltaDir, err := os.MkdirTemp(c.opts.InternalDirectory, "deltas-*")
	if err != nil {
		return false, err
	}
	defer func() {
		_ = os.RemoveAll(deltaDir)
	}()
	_, _, deltas, err := c.reg.ResolveAndLoadToPath(res.DeltaImage, deltaDir)
	if err != nil {
		return false, err
	}
	// patch output directory in place
	for _, d := range deltas {
		err := c.patchArtifact(d)
		if err != nil {
			return false, err
		}
	}
	dirHash, err := dirhash.HashDir(c.opts.OutputDirectory, "", dirhash.Hash1)
	if err != nil {
		return false, err
	}
	dirHashDigest := digest.Digest(dirHash)
	err = c.state.ModifyState(func(u *updaterstate.State) error {
		return u.SetArtifactState(c.opts.OutputDirectory, res.TargetImage, dirHashDigest)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) patchArtifact(d fetcher.LoadResult) error {
	p, err := c.getPatcherChoice(&d.D, c.patcherTmpDir)
	if err != nil {
		return err
	}
	fp, err := os.Open(d.Path)
	if err != nil {
		return err
	}
	decompressedPatch, err := p.Decompress(fp)
	if err != nil {
		return err
	}
	// digest is not provided, however we already verified a digest while fetching the deltas
	// PatchFilesystem() takes care of robust file swapping
	err = p.PatchFilesystem(c.opts.OutputDirectory, decompressedPatch, nil)
	if err != nil {
		return err
	}
	_ = fp.Close()
	_ = os.Remove(d.Path)
	return nil
}

func (c *Client) pullFullImage(targetImage string) (bool, error) {
	log.Info("attempting to load full artifact")
	repoName, _, _, err := ociutils.ParseOciImageString(targetImage)
	if err != nil {
		return false, err
	}
	intermediateDir, err := os.MkdirTemp(c.opts.InternalDirectory, "intermediate-*")
	if err != nil {
		return false, err
	}
	defer func() {
		_ = os.RemoveAll(intermediateDir)
	}()
	// we might receive archives, pull them to an intermediate directory
	mfD, _, res, err := c.reg.ResolveAndLoadToPath(targetImage, intermediateDir)
	if err != nil {
		return false, err
	}
	// this directory gets filled with all artifacts and replaces the output directory once completed
	extractDir, err := os.MkdirTemp(c.opts.InternalDirectory, "extract-*")
	if err != nil {
		return false, err
	}
	err = os.Chmod(extractDir, c.opts.OutputDirPermissions)
	if err != nil {
		return false, err
	}
	defer func() {
		// remove the extract dir if it has not been removed yet
		_ = os.RemoveAll(extractDir)
	}()
	// extract archives into the extractDir or move non-archive files there directly
	for _, r := range res {
		if r.D.Annotations[constants.OrasContentUnpack] == "true" {
			err := tarutils.ExtractCompressedTar(extractDir, "", r.Path, nil, gzip.NewDecompressor())
			if err != nil {
				return false, err
			}
			_ = os.RemoveAll(r.Path)
			continue
		}
		// TODO: also handle compressed artifacts that are not archives
		targetPath := path.Join(extractDir, path.Base(r.Path))
		log.Debugf("attempted to move to %q", targetPath)
		err = fileutils.ReplaceFile(r.Path, targetPath)
		if err != nil {
			return false, err
		}
	}
	// replace output directory once we fully populated the directory
	err = fileutils.ReplaceDirectory(extractDir, c.opts.OutputDirectory)
	if err != nil {
		return false, err
	}
	// save the current version to the state
	currentImage := fmt.Sprintf("%s@%s", repoName, mfD.Digest.String())
	dirHash, err := dirhash.HashDir(c.opts.OutputDirectory, "", dirhash.Hash1)
	if err != nil {
		return false, err
	}
	dirHashDigest := digest.Digest(dirHash)
	err = c.state.ModifyState(func(u *updaterstate.State) error {
		return u.SetArtifactState(c.opts.OutputDirectory, currentImage, dirHashDigest)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
