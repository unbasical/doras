package e2e

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras-server/internal/pkg/utils/logutils"
	testutils2 "github.com/unbasical/doras-server/internal/pkg/utils/testutils"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"github.com/unbasical/doras-server/configs"
	"github.com/unbasical/doras-server/internal/pkg/core"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/pkg/client/edgeapi"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

func Test_ReadAndApplyDelta(t *testing.T) {
	ctx := context.Background()
	logutils.SetupTestLogging()

	fromDataBsdiff := []byte("foo")
	toDataBsdiff := []byte("bar")
	deltaWantBsdiff, err := bsdiff.Bytes(fromDataBsdiff, toDataBsdiff)
	if err != nil {
		t.Fatal(err)
	}
	fromDataTarDiff := fileutils.ReadOrPanic("../test-files/from.tar.gz")
	toDataTarDiff := fileutils.ReadOrPanic("../test-files/to.tar.gz")
	deltaWantTarDiff := fileutils.ReadOrPanic("../test-files/delta.patch.tardiff")

	// decompress tar because the ApplyDelta has an uncompressed tar as the output
	gzr, err := gzip.NewReader(bytes.NewBuffer(toDataTarDiff))
	if err != nil {
		t.Fatal(err)
	}
	toDataTarDiff, err = io.ReadAll(gzr)
	if err != nil {
		t.Fatal(err)
	}

	regUri := testutils2.LaunchRegistry(ctx)

	host := "localhost:8081"
	config := configs.DorasServerConfig{
		Sources: map[string]configs.OrasSourceConfiguration{
			regUri + "/artifacts": {
				EnableHTTP: false,
			},
		},
		Storage: configs.StorageConfiguration{
			URL:        regUri,
			EnableHTTP: true,
		},
		Host: host,
	}
	dorasApp := core.Doras{}
	go dorasApp.Init(config).Start()

	reg, err := remote.NewRegistry(regUri)
	if err != nil {
		t.Fatal(err)
	}
	reg.PlainHTTP = true
	repoArtifacts, err := reg.Repository(ctx, "artifacts")
	if err != nil {
		t.Fatal(err)
	}

	// populate the oras internal registry with files from which deltas will be created
	tempDir := t.TempDir()
	tag1Bsdiff := "v1-bsdiff"
	tag2Bsdiff := "v2-bsdiff"
	tag1Tardiff := "v1-tardiff"
	tag2Tardiff := "v2-tardiff"
	store, err := testutils2.StorageFromFiles(ctx,
		tempDir,
		[]testutils2.FileDescription{
			{
				Name: "test-artifact",
				Data: fromDataBsdiff,
				Tag:  tag1Bsdiff,
			},
			{
				Name: "test-artifact",
				Data: toDataBsdiff,
				Tag:  tag2Bsdiff,
			},
			{
				Name:        "test-artifact",
				Data:        fromDataTarDiff,
				Tag:         tag1Tardiff,
				NeedsUnpack: true,
			},
			{
				Name:        "test-artifact",
				Data:        toDataTarDiff,
				Tag:         tag2Tardiff,
				NeedsUnpack: true,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	tags := []string{tag1Bsdiff, tag2Bsdiff, tag1Tardiff, tag2Tardiff}
	descriptors := lo.Reduce(tags, func(agg map[string]v1.Descriptor, tag string, _ int) map[string]v1.Descriptor {
		rootDescriptor, err := oras.Copy(ctx, store, tag, repoArtifacts, tag, oras.DefaultCopyOptions)
		if err != nil {
			t.Fatal(err)
		}
		agg[tag] = rootDescriptor
		return agg
	}, make(map[string]v1.Descriptor))

	// make sure server has launched
	for {
		res, err := http.DefaultClient.Get(fmt.Sprintf("http://%s/api/v1/ping", host))
		if err != nil {
			t.Error(err)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			t.Error(err)
			continue
		}
		if strings.Contains(string(resBody), "pong") {
			break
		}
		err = res.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}

	edgeClient, err := edgeapi.NewEdgeClient(fmt.Sprintf("http://%s", host), regUri, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name       string
		from       string
		fromDesc   v1.Descriptor
		to         string
		fromReader io.Reader
		toReader   io.Reader
		want       []byte
	}{
		{
			name:       "bsdiff",
			from:       tag1Bsdiff,
			fromDesc:   descriptors["v1-bsdiff"],
			fromReader: bytes.NewBuffer(fromDataBsdiff),
			to:         tag2Bsdiff,
			toReader:   bytes.NewBuffer(toDataBsdiff),
			want:       deltaWantBsdiff,
		},
		{
			name:       "tardiff",
			from:       tag1Tardiff,
			fromDesc:   descriptors["v1-tardiff"],
			fromReader: bytes.NewBuffer(fromDataTarDiff),
			to:         tag2Tardiff,
			toReader:   bytes.NewBuffer(toDataTarDiff),
			want:       deltaWantTarDiff,
		},
		{
			name:       "same bsdiff again to test repeated requests",
			from:       tag1Bsdiff,
			fromDesc:   descriptors["v1-bsdiff"],
			fromReader: bytes.NewBuffer(fromDataBsdiff),
			to:         tag2Bsdiff,
			toReader:   bytes.NewBuffer(toDataBsdiff),
			want:       deltaWantBsdiff,
		},
	} {
		imageFrom := fmt.Sprintf("%s/%s:%s", regUri, "artifacts", tt.from)
		imageTo := fmt.Sprintf("%s/%s:%s", regUri, "artifacts", tt.to)
		imageFromDigest := fmt.Sprintf("%s/%s@sha256:%s", regUri, "artifacts", tt.fromDesc.Digest.Encoded())
		// read delta from sever
		_, _, _, err := edgeClient.ReadDeltaAsStream(imageFrom, imageTo, nil)
		if err == nil {
			t.Fatal(err)
		}
		_, algo, rc, err := edgeClient.ReadDeltaAsStream(imageFromDigest, imageTo, nil)
		if err != nil {
			t.Fatal(err)
		}
		deltaGot, err := io.ReadAll(rc)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(deltaGot, tt.want) {
			t.Errorf("got:\n%x\nwant:\n%x", deltaGot, tt.want)
		}

		// apply the requested data
		patchedReader, err := delta.ApplyDelta(
			algo,
			bytes.NewReader(deltaGot),
			tt.fromReader,
		)
		if err != nil {
			t.Error(err)
			continue
		}
		patchedData, err := io.ReadAll(patchedReader)
		if err != nil {
			t.Fatal(err)
		}
		toWant, err := io.ReadAll(tt.toReader)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(patchedData, toWant) {
			t.Errorf("got:\n%x\nwant:\n%x", patchedData, toWant)
		}
	}

}
