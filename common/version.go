package common

import _ "embed"

//go:embed version.txt
var version string

// Version returns the current version of Doras.
func Version() string {
	return version
}
