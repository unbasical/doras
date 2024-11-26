package e2e

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"github.com/unbasical/doras-server/configs"
	"github.com/unbasical/doras-server/internal/pkg/core"
	"github.com/unbasical/doras-server/internal/pkg/fileutils"
	"github.com/unbasical/doras-server/internal/pkg/logutils"
	"github.com/unbasical/doras-server/internal/pkg/testutils"
	"github.com/unbasical/doras-server/pkg/client/edgeapi"
	"io"
	"net/http"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"strings"
	"testing"
	"time"
)

func Test_ReadDelta(t *testing.T) {
	fromDataBsdiff := []byte("foo")
	toDataBsdiff := []byte("bar")
	deltaWantBsdiff, err := bsdiff.Bytes(fromDataBsdiff, toDataBsdiff)
	if err != nil {
		t.Fatal(err)
	}
	fromDataTarDiff := fileutils.ReadOrPanic("../../internal/pkg/delta/test-files/from.tar.gz")
	toDataTarDiff := fileutils.ReadOrPanic("../../internal/pkg/delta/test-files/to.tar.gz")
	deltaWantTarDiff := fileutils.ReadOrPanic("../../internal/pkg/delta/test-files/delta.patch.tardiff")

	logutils.SetupTestLogging()
	ctx := context.Background()

	regUri := testutils.LaunchRegistry(ctx)

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

	tempDir := t.TempDir()
	tag1Bsdiff := "v1-bsdiff"
	tag2Bsdiff := "v2-bsdiff"
	tag1Tardiff := "v1-tardiff"
	tag2Tardiff := "v2-tardiff"
	store, err := testutils.StorageFromFiles(ctx,
		tempDir,
		[]testutils.FileDescription{
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
	tags := []string{tag1Bsdiff, tag2Bsdiff, tag1Tardiff, tag2Tardiff}
	_ = lo.Reduce(tags, func(agg map[string]v1.Descriptor, tag string, _ int) map[string]v1.Descriptor {
		descriptor, err := oras.Copy(ctx, store, tag, repoArtifacts, tag, oras.DefaultCopyOptions)
		if err != nil {
			t.Fatal(err)
		}
		agg[tag] = descriptor
		return agg
	}, make(map[string]v1.Descriptor))

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
		name string
		from string
		to   string
		want []byte
	}{
		{name: "bsdiff", from: tag1Bsdiff, to: tag2Bsdiff, want: deltaWantBsdiff},
		{name: "tardiff", from: tag1Tardiff, to: tag2Tardiff, want: deltaWantTarDiff},
	} {
		imageFrom := fmt.Sprintf("%s/%s:%s", regUri, "artifacts", tt.from)
		imageTo := fmt.Sprintf("%s/%s:%s", regUri, "artifacts", tt.to)

		r, err := edgeClient.ReadDelta(imageFrom, imageTo, nil)
		if err != nil {
			t.Error(err)
		}
		deltaGot, err := io.ReadAll(r)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(deltaGot, tt.want) {
			t.Errorf("got:\n%x\nwant:\n%x", deltaGot, tt.want)
		}
	}

}
