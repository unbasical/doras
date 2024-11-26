package testutils

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func LaunchRegistry(ctx context.Context) string {
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
