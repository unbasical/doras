package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/unbasical/doras-server/internal/pkg/logutils"
	"github.com/unbasical/doras-server/internal/pkg/testutils"
	"github.com/unbasical/doras-server/pkg/client/cloudapi"
	"github.com/unbasical/doras-server/pkg/client/edgeapi"

	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/configs"
	"github.com/unbasical/doras-server/internal/pkg/core"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

func Test_AddAndLoadDelta(t *testing.T) {

	logutils.SetupTestLogging()
	ctx := context.Background()
	uriSrc := testutils.LaunchRegistry(ctx)
	uriDst := testutils.LaunchRegistry(ctx)
	repoSrc, err := remote.NewRepository(uriSrc + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	repoSrc.PlainHTTP = true
	regDst, err := remote.NewRegistry(uriDst)
	if err != nil {
		t.Fatal(err)
	}
	regDst.PlainHTTP = true

	tempDir := t.TempDir()
	store, err := file.New(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	helloPath := path.Join(tempDir, "hello")
	err = os.WriteFile(helloPath, []byte("Hello World!"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	// 1. Add files to the file store^
	mediaType := "application/vnd.test.file"
	fileNames := []string{"hello"}
	fileDescriptors := make([]v1.Descriptor, 0, len(fileNames))
	for _, name := range fileNames {
		fileDescriptor, err := store.Add(ctx, name, mediaType, path.Join(tempDir, name))
		if err != nil {
			t.Fatal(err)
		}
		fileDescriptors = append(fileDescriptors, fileDescriptor)
		fmt.Printf("file descriptor for %s: %v\n", name, fileDescriptor)
	}

	// 2. Pack the files and tag the packed manifest
	artifactType := "application/vnd.test.artifact"
	opts := oras.PackManifestOptions{
		Layers: fileDescriptors,
	}
	manifestDescriptor, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, artifactType, opts)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("manifest descriptor:", manifestDescriptor)

	tag := "v1"
	if err = store.Tag(ctx, manifestDescriptor, tag); err != nil {
		t.Fatal(err)
	}
	// populate source repository
	_, err = oras.Copy(ctx, store, "v1", repoSrc, "v1", oras.DefaultCopyOptions)
	if err != nil {
		t.Fatal(err)
	}
	config := configs.DorasServerConfig{
		Sources: map[string]configs.OrasSourceConfiguration{
			uriSrc + "/hello": {
				EnableHTTP: false,
			},
		},
		Storage: configs.StorageConfiguration{
			URL:        uriDst,
			EnableHTTP: true,
		},
	}
	dorasApp := core.Doras{}

	go dorasApp.Init(config).Start()
	log.Info(repoSrc, regDst)
	for {
		res, err := http.DefaultClient.Get("http://localhost:8080/api/v1/ping")
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
	orasClient := cloudapi.NewClient("http://localhost:8080")
	repoPath, tag, err := orasClient.CreateArtifactFromOCIReference(uriSrc + "/hello:v1")
	if err != nil {
		t.Fatal(err)
	}
	tempDir = t.TempDir()

	edgeClient, err := edgeapi.NewEdgeClient("http://localhost:8080", uriDst, true)
	if err != nil {
		t.Fatal(err)
	}

	err = edgeClient.LoadArtifact(uriDst+"/"+repoPath+":"+tag, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	helloPath = path.Join(tempDir, "hello")

	data, err := os.ReadFile(helloPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "Hello World!" {
		t.Fatalf("unexpected file content")
	}
}
