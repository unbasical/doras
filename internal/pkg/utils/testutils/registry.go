package testutils

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// LaunchRegistry sets up testcontainer based on the `registry` image and returns the server's URI.
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

	// Create a mapping to the server's http interface.
	mappedPort, err := container.MappedPort(ctx, "5000")
	if err != nil {
		panic(err)
	}

	// Extract the host and port.
	hostIP, err := container.Host(ctx)
	if err != nil {
		panic(err)
	}
	port := mappedPort.Port()

	// Create URI of the mapped socket address where the server is available.
	uri := fmt.Sprintf("%s:%s", hostIP, port)
	return uri
}
