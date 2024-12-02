// Package updater implements an ergonomic API for delta updates based on the doras client.
package updater

import (
	"context"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"io"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/pkg/client/edgeapi"
	"oras.land/oras-go/v2"
)

type Client struct {
	edgeClient  edgeapi.Client
	orasStorage oras.Target
}

func patchStorage(src oras.ReadOnlyTarget, dst oras.Target, old, new v1.Descriptor, deltaFile io.Reader) error {
	oldReader, err := src.Fetch(context.Background(), old)
	if err != nil {
		return err
	}
	newReader, err := delta.ApplyDeltaWithBlobDescriptor(new, oldReader, deltaFile)
	if err != nil {
		return err
	}
	defer funcutils.PanicOrLogOnErr(oldReader.Close, false, "failed to close blob reader")
	err = dst.Push(context.Background(), new, newReader)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) patchFile(identifier v1.Descriptor, path string, deltaFile io.Writer) (io.Writer, error) {
	panic("")
}
