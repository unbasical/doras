package main

import (
	"context"
	"github.com/alecthomas/kong"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/utils/logutils"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type cliArgs struct {
	DockerConfigFilePath string `help:"Path to the docker config file which is used to access registry credentials." default:"~/.docker/config.json" env:"DOCKER_CONFIG_FILE_PATH"`
	LogLevel             string `help:"Log level." default:"info" enum:"debug,info,warn,error" env:"DORAS_LOG_LEVEL"`
	LogFormat            string `help:"Log format." default:"text" enum:"json,text" env:"DORAS_LOG_FORMAT"`
	InsecureAllowHTTP    bool   `help:"Allow INSECURE HTTP connections." default:"false" env:"DORAS_INSECURE_ALLOW_HTTP"`
	Remote               string `help:"The URL of the Doras server." default:"localhost:8080" env:"DORAS_SERVER_URL"`
	Push                 struct {
		Overwrite    bool   `help:"Overwrite existing artifact if it exists." default:"false"`
		Compress     string `help:"Compress artifact before uploading." default:"gzip" enum:"zstd,gzip,none"`
		ArchiveFiles bool   `help:"Archive artifact before uploading." default:"false"`
		Image        string `arg:"" name:"image" help:"Target image/repository where the artifact will be published."`
		Path         string `arg:"" name:"path" help:"Path of the artifact that should be uploaded (single file or directory)"`
	} `cmd:"" help:"Upload artifact to a registry."`
	Pull struct {
		Image  string `arg:"" name:"image" help:"Target image/repository which is pulled."`
		Output string `name:"path" help:"Output directory." type:"path" default:"."`
	} `cmd:"" name:"pull" help:"Pull an artifact from a registry, uses delta updates if possible."`
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
	default:
		log.Fatalf("Unknown command: %v", cliCtx.Command())
	}
	if err != nil {
		log.WithError(err).Fatal("Command failed")
	}
}

// getCredentialFunc loads the configured docker credential store and returns an auth.CredentialFunc.
func (args *cliArgs) getCredentialFunc() (auth.CredentialFunc, error) {
	credentialStore, err := credentials.NewStore(args.DockerConfigFilePath, credentials.StoreOptions{
		AllowPlaintextPut:        false,
		DetectDefaultNativeStore: true,
	})
	if err != nil {
		return nil, err
	}
	return credentials.Credential(credentialStore), nil
}
