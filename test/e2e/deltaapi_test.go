package e2e

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"fmt"
	log "github.com/sirupsen/logrus"
	gzip2 "github.com/unbasical/doras/internal/pkg/compression/gzip"
	delta2 "github.com/unbasical/doras/pkg/algorithm/delta"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/unbasical/doras/internal/pkg/compression/zstd"
	"github.com/unbasical/doras/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras/pkg/algorithm/compression"

	bsdiff2 "github.com/unbasical/doras/internal/pkg/delta/bsdiff"
	"github.com/unbasical/doras/internal/pkg/delta/tardiff"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/internal/pkg/utils/logutils"
	testutils2 "github.com/unbasical/doras/internal/pkg/utils/testutils"

	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"github.com/unbasical/doras/configs"
	"github.com/unbasical/doras/internal/pkg/core"
	"github.com/unbasical/doras/pkg/client/edgeapi"
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
	fromDataBsdiffBig := make([]byte, 0xfff)
	_, err = rand.Read(fromDataBsdiffBig)
	if err != nil {
		t.Fatal(err)
	}
	toDataBsdiffBig := make([]byte, 0xfff)
	_, err = rand.Read(toDataBsdiffBig)
	if err != nil {
		t.Fatal(err)
	}
	deltaWantBsdiffBig, err := bsdiff.Bytes(fromDataBsdiffBig, toDataBsdiffBig)
	if err != nil {
		t.Fatal(err)
	}
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
	serverConfig := configs.ServerConfig{
		ConfigFile: configs.ServerConfigFile{},
		CliOpts:    configs.CLI{HTTPPort: 8081, Host: "localhost", LogLevel: "debug", InsecureAllowHTTP: true},
	}
	dorasServer := core.New(serverConfig)
	go dorasServer.Start()

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
	tag1BsdiffBig := "v1-bsdiff-big"
	tag2BsdiffBig := "v2-bsdiff-big"
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
				Name: "test-artifact",
				Data: fromDataBsdiffBig,
				Tag:  tag1BsdiffBig,
			},
			{
				Name: "test-artifact",
				Data: toDataBsdiffBig,
				Tag:  tag2BsdiffBig,
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
	tags := []string{tag1Bsdiff, tag2Bsdiff, tag1Tardiff, tag2Tardiff, tag1BsdiffBig, tag2BsdiffBig}
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

	edgeClient, err := edgeapi.NewEdgeClient(fmt.Sprintf("http://%s", host), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name               string
		from               string
		fromDesc           v1.Descriptor
		to                 string
		fromReader         io.Reader
		toReader           io.Reader
		acceptedAlgorithms []string
		want               []byte
		wantAlgo           string
	}{
		{
			name:               "bsdiff (no compression)",
			from:               tag1Bsdiff,
			fromDesc:           descriptors["v1-bsdiff"],
			fromReader:         bytes.NewBuffer(fromDataBsdiff),
			to:                 tag2Bsdiff,
			toReader:           bytes.NewBuffer(toDataBsdiff),
			want:               deltaWantBsdiff,
			wantAlgo:           "bsdiff",
			acceptedAlgorithms: []string{"bsdiff"},
		},
		{
			name:               "same bsdiff again to test repeated requests",
			from:               tag1Bsdiff,
			fromDesc:           descriptors["v1-bsdiff"],
			fromReader:         bytes.NewBuffer(fromDataBsdiff),
			to:                 tag2Bsdiff,
			toReader:           bytes.NewBuffer(toDataBsdiff),
			wantAlgo:           "bsdiff",
			acceptedAlgorithms: []string{"bsdiff"},
			want:               deltaWantBsdiff,
		},
		{
			name:               "bsdiff (gzip compression)",
			from:               tag1Bsdiff,
			fromDesc:           descriptors["v1-bsdiff"],
			fromReader:         bytes.NewBuffer(fromDataBsdiff),
			to:                 tag2Bsdiff,
			toReader:           bytes.NewBuffer(toDataBsdiff),
			want:               deltaWantBsdiff,
			wantAlgo:           "bsdiff+gzip",
			acceptedAlgorithms: []string{"bsdiff", "gzip"},
		},
		{
			name:               "bsdiff (zstd compression)",
			from:               tag1Bsdiff,
			fromDesc:           descriptors["v1-bsdiff"],
			fromReader:         bytes.NewBuffer(fromDataBsdiff),
			to:                 tag2Bsdiff,
			toReader:           bytes.NewBuffer(toDataBsdiff),
			want:               deltaWantBsdiff,
			wantAlgo:           "bsdiff+zstd",
			acceptedAlgorithms: []string{"bsdiff", "zstd"},
		},
		{
			name:               "tardiff (no compression)",
			from:               tag1Tardiff,
			fromDesc:           descriptors["v1-tardiff"],
			fromReader:         bytes.NewBuffer(fromDataTarDiff),
			to:                 tag2Tardiff,
			toReader:           bytes.NewBuffer(toDataTarDiff),
			want:               deltaWantTarDiff,
			wantAlgo:           "tardiff",
			acceptedAlgorithms: []string{"tardiff", "zstd"},
		},
		{
			name:               "bsdiff big (no compression)",
			from:               tag1BsdiffBig,
			fromDesc:           descriptors["v1-bsdiff-big"],
			fromReader:         bytes.NewBuffer(fromDataBsdiffBig),
			to:                 tag2BsdiffBig,
			toReader:           bytes.NewBuffer(toDataBsdiffBig),
			want:               deltaWantBsdiffBig,
			wantAlgo:           "bsdiff",
			acceptedAlgorithms: []string{"bsdiff"},
		},
		{
			name:               "bsdiff big (gzip compression)",
			from:               tag1BsdiffBig,
			fromDesc:           descriptors["v1-bsdiff-big"],
			fromReader:         bytes.NewBuffer(fromDataBsdiffBig),
			to:                 tag2BsdiffBig,
			toReader:           bytes.NewBuffer(toDataBsdiffBig),
			want:               deltaWantBsdiffBig,
			wantAlgo:           "bsdiff+gzip",
			acceptedAlgorithms: []string{"bsdiff", "gzip"},
		},
		{
			name:               "bsdiff big (zstd compression)",
			from:               tag1BsdiffBig,
			fromDesc:           descriptors["v1-bsdiff-big"],
			fromReader:         bytes.NewBuffer(fromDataBsdiffBig),
			to:                 tag2BsdiffBig,
			toReader:           bytes.NewBuffer(toDataBsdiffBig),
			want:               deltaWantBsdiffBig,
			wantAlgo:           "bsdiff+zstd",
			acceptedAlgorithms: []string{"bsdiff", "zstd"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			imageFrom := fmt.Sprintf("%s/%s:%s", regUri, "artifacts", tt.from)
			imageTo := fmt.Sprintf("%s/%s:%s", regUri, "artifacts", tt.to)
			imageFromDigest := fmt.Sprintf("%s/%s@%s", regUri, "artifacts", tt.fromDesc.Digest.String())
			// read delta from sever
			_, _, _, err := edgeClient.ReadDeltaAsStream(imageFrom, imageTo, tt.acceptedAlgorithms)
			if err == nil {
				t.Fatal(err)
			}
			_, algo, rc, err := edgeClient.ReadDeltaAsStream(imageFromDigest, imageTo, tt.acceptedAlgorithms)
			if err != nil {
				t.Fatal(err)
			}
			if algo != tt.wantAlgo {
				t.Fatalf("got algo = %v, want %v", algo, tt.wantAlgo)
			}
			var decompressor compression.Decompressor
			var patcher delta2.Patcher
			switch algo {
			case "tardiff+zstd":
				patcher = tardiff.NewPatcher()
				decompressor = zstd.NewDecompressor()
			case "tardiff+gzip":
				patcher = tardiff.NewPatcher()
				decompressor = gzip2.NewDecompressor()
			case "bsdiff+zstd":
				patcher = bsdiff2.NewPatcher()
				decompressor = zstd.NewDecompressor()
			case "bsdiff+gzip":
				patcher = bsdiff2.NewPatcher()
				decompressor = gzip2.NewDecompressor()
			case "tardiff":
				patcher = tardiff.NewPatcher()
				decompressor = compressionutils.NewNopDecompressor()
			case "bsdiff":
				patcher = bsdiff2.NewPatcher()
				decompressor = compressionutils.NewNopDecompressor()
			default:
				t.Fatalf("unknown algorithm %s", algo)
			}
			deltaGot, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			decompressedDelta, err := decompressor.Decompress(bytes.NewReader(deltaGot))
			if err != nil {
				t.Fatal(err)
			}
			deltaGot, err = io.ReadAll(decompressedDelta)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(deltaGot, tt.want) {
				t.Fatalf("%s: got:\n%x\nwant:\n%x", tt.name, deltaGot, tt.want)
			}

			// apply the requested data
			patchedReader, err := patcher.Patch(
				tt.fromReader,
				bytes.NewReader(deltaGot),
			)
			if err != nil {
				t.Fatal(err)
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
				t.Errorf("%s: got:\n%x\nwant:\n%x", tt.name, patchedData, toWant)
			}
		})
	}
	t.Run("test concurrent requests", func(t *testing.T) {
		imageFrom := fmt.Sprintf("%s/%s@sha256:%s", regUri, "artifacts", descriptors["v1-bsdiff"].Digest.Encoded())
		imageTo := fmt.Sprintf("%s/%s:%s", regUri, "artifacts", tag2Bsdiff)
		wg := sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				_, err := edgeClient.ReadDelta(imageFrom, imageTo, nil)
				if err != nil {
					t.Error(err)
				}
				wg.Done()
			}()
		}
		wg.Wait()
	})
	// Gracefully shut down the server with a timeout
	gracefulPeriod := 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), gracefulPeriod)
	log.Infof("shutting down server with a graceful period of %d Seconds ...", gracefulPeriod/time.Second)
	defer cancel()
	if err := dorasServer.Stop(ctx); err != nil {
		log.WithError(err).Info("stopping Doras server had error")
		return
	}
	log.Println("Server exited gracefully")
}
