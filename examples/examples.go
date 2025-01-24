package examples

import (
	_ "embed"
	"strings"
)

//go:embed doras-server/config.yaml
var dorasExampleConfig string

// DorasExampleConfig returns an example config for the doras server.
func DorasExampleConfig() string {
	return strings.Clone(dorasExampleConfig)
}
