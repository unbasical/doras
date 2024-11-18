package e2e

import (
	"context"
	"fmt"
	"time"

	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/unbasical/doras-server/configs"
	"github.com/unbasical/doras-server/internal/pkg/core"
	"github.com/unbasical/doras-server/pkg/client"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

func Test_everything(t *testing.T) {

	log.SetLevel(log.DebugLevel)
	log.SetFormatter(UTCFormatter{Formatter: &log.TextFormatter{FullTimestamp: true}})
	ctx := context.Background()
	uriSrc := getRegistryURI(ctx)
	uriDst := getRegistryURI(ctx)
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
			panic(err)
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
		panic(err)
	}
	fmt.Println("manifest descriptor:", manifestDescriptor)

	tag := "v1"
	if err = store.Tag(ctx, manifestDescriptor, tag); err != nil {
		panic(err)
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
	orasClient := client.NewCloudClient("http://localhost:8080")
	repoPath, tag, err := orasClient.CreateArtifactFromOCIReference(uriSrc + "/hello:v1")
	if err != nil {
		panic(err)
	}
	tempDir = t.TempDir()

	edgeClient, err := client.NewEdgeClient("http://localhost:8080", uriDst, true)
	if err != nil {
		t.Fatal(err)
	}

	err = edgeClient.LoadArtifact(uriDst+"/"+repoPath+":"+tag, tempDir)
	if err != nil {
		panic(err)
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

func getRegistryURI(ctx context.Context) string {
	req := testcontainers.ContainerRequest{
		Image:        "registry:2.8",
		ExposedPorts: []string{"5000/tcp"},
		WaitingFor:   wait.ForHTTP("/").WithPort("5000/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		panic(err)
	}

	mappedPort, err := container.MappedPort(ctx, "5000")
	if err != nil {
		panic(err)
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		panic(err)
	}

	uri := fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())
	return uri
}

type UTCFormatter struct {
	log.Formatter
}

func (u UTCFormatter) Format(e *log.Entry) ([]byte, error) {
	e.Time = e.Time.UTC()
	return u.Formatter.Format(e)
}
