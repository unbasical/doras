package configs

import (
	"github.com/unbasical/doras/examples"
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func Test_DorasServerConfigExample(t *testing.T) {
	dorasConfigYAML := examples.DorasExampleConfig()
	var cfg ServerConfigFile
	decoder := yaml.NewDecoder(strings.NewReader(dorasConfigYAML))
	decoder.KnownFields(true)
	err := decoder.Decode(&cfg)
	if err != nil {
		t.Fatalf("Error parsing doras config file: %v", err)
	}
}
