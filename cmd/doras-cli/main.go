package main

import (
	"context"
	"github.com/alecthomas/kong"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/utils/logutils"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type cliArgs struct {
	DockerConfigFilePath string `help:"Path to the docker config file which is used to access registry credentials." default:"~/.docker/config.json" env:"DOCKER_CONFIG_FILE_PATH"`
	LogLevel             string `help:"Log level." default:"info" enum:"debug,info,warn,error" env:"DORAS_LOG_LEVEL"`
	LogFormat            string `help:"Log format." default:"text" enum:"json,text" env:"DORAS_LOG_FORMAT"`
	InsecureAllowHTTP    bool   `help:"Allow INSECURE HTTP connections." default:"false" env:"DORAS_INSECURE_ALLOW_HTTP"`
	Remote               string `help:"The URL of the Doras server." default:"http://localhost:8080" env:"DORAS_SERVER_URL"`
	Push                 struct {
		Compress     string `help:"Compress artifact before uploading." default:"gzip" enum:"zstd,gzip,none"`
		ArchiveFiles bool   `help:"Archive artifact before uploading." default:"false"`
		Image        string `arg:"" name:"image" help:"Target image/repository where the artifact will be published."`
		Path         string `arg:"" name:"path" help:"Path of the artifact that should be uploaded (single file or directory)"`
	} `cmd:"" help:"Upload artifact to a registry."`
	Pull struct {
		Image             string   `arg:"" name:"image" help:"Target image/repository which is pulled."`
		Output            string   `help:"Output directory." type:"path" default:"."`
		Async             bool     `help:"Do not block until the delta is created." default:"false"`
		InternalDir       string   `help:"Doras internal directory." type:"path" default:"~/.local/share/doras"`
		AcceptedAlgorithm []string `help:"Select algorithms which are accepted for deltas."`
	} `cmd:"" name:"pull" help:"Pull an artifact from a registry, uses readDelta updates if possible."`
	ReadDelta struct {
		From              string   `help:"From which image the delta will be built."`
		To                string   `help:"To which image the delta will be built."`
		Async             bool     `help:"Do not block until the delta is created." default:"false"`
		AcceptedAlgorithm []string `help:"Select algorithms which are accepted for deltas."`
	} `cmd:"" help:"Request a delta image from the Doras server."`
}

func main() {
	ctx := context.Background()
	// parse args
	args := cliArgs{}
	cliCtx := kong.Parse(&args)

	// Setup logging.
	logutils.SetLogLevel(args.LogLevel)
	logutils.SetLogFormat(args.LogFormat)

	var err error
	switch cliCtx.Command() {
	case "push <image> <path>":
		err = args.push(ctx)
	case "pull <image>":
		err = args.pull(ctx)
	case "read-delta":
		err = args.readDelta(ctx)
	default:
		log.Fatalf("Unknown command: %v", cliCtx.Command())
	}
	if err != nil {
		log.WithError(err).Fatal("Command failed")
	}
}

// getCredentialFunc loads the configured docker credential store and returns an auth.CredentialFunc.
func (args *cliArgs) getCredentialFunc() (auth.CredentialFunc, error) {
	log.Debug("attempting to load credentials using provided docker config file")
	credentialStore, err := credentials.NewStore(args.DockerConfigFilePath, credentials.StoreOptions{
		AllowPlaintextPut:        false,
		DetectDefaultNativeStore: true,
	})
	if err != nil {
		log.WithError(err).Debug("load credential store from docker config")
		return nil, err
	}
	log.Debug("loaded credential store successfully")
	return credentials.Credential(credentialStore), nil
}
