package main

import (
	"github.com/alecthomas/kong"
	"github.com/unbasical/doras-server/internal/pkg/utils/logutils"
)

type cliArgs struct {
	DockerConfigFilePath string `help:"Path to the docker config file which is used to access registry credentials." default:"~/.docker/config.json" env:"DOCKER_CONFIG_FILE_PATH"`
	LogLevel             string `help:"Log level." default:"info" enum:"debug,info,warn,error" env:"DORAS_LOG_LEVEL"`
	LogFormat            string `help:"Log format." default:"text" enum:"json,text" env:"DORAS_LOG_FORMAT"`
	InsecureAllowHTTP    bool   `help:"Allow INSECURE HTTP connections." default:"false" env:"DORAS_INSECURE_ALLOW_HTTP"`
	DorasServerURL       string `help:"The URL of the Doras server." default:"localhost:8080" env:"DORAS_SERVER_URL"`
	Push                 struct {
		Overwrite    bool   `help:"Overwrite existing artifact if it exists." default:"false"`
		Compress     string `help:"Compress artifact before uploading." default:"zstd" enum:"zstd,gzip,false"`
		ArchiveFiles bool   `help:"Archive artifact before uploading." default:"false"`
		Image        string `arg:"" name:"image" help:"Target image/repository where the artifact will be published."`
		Path         string `arg:"" name:"path" help:"Path of the artifact that should be uploaded (single file or directory)" type:"path"`
	} `cmd:"" help:"Upload artifact to a registry."`
	Pull struct {
		Image string `arg:"" name:"image" help:"Target image/repository which is pulled."`
		Path  string `arg:"" name:"path" help:"Output directory." type:"path"`
	} `cmd:"" name:"pull" help:"Pull an artifact from a registry, uses delta updates if possible."`
}

func main() {
	// parse args
	args := cliArgs{}
	ctx := kong.Parse(&args)

	// Setup logging.
	logutils.SetLogLevel(args.LogLevel)
	logutils.SetLogFormat(args.LogFormat)

	var err error
	switch ctx.Command() {
	case "push <image> <path>":
		err = args.push()
	case "pull <image> <path>":
		err = args.pull()
	default:
		panic(ctx.Command())
	}
	if err != nil {
		panic(err)
	}
}
