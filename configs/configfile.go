package configs

// ServerConfig is the data structure to configure a Doras s
type ServerConfig struct {
	ConfigFile ServerConfigFile
	CliOpts    CLI
}

// CLI is the struct to parse the command line parameters or environment variables.
type CLI struct {
	HTTPPort             uint16 `help:"HTTP port to listen on." default:"8080" env:"DORAS_HTTP_PORT"`
	Host                 string `help:"Hostname to listen on." default:"127.0.0.1" env:"DORAS_HOST"`
	ConfigFilePath       string `help:"Path to the Doras server config file." env:"DORAS_CONFIG_FILE_PATH"`
	DockerConfigFilePath string `help:"Path to the docker config file which is used to access registry credentials." default:"~/.docker/config.json" env:"DOCKER_CONFIG_FILE_PATH"`
	LogLevel             string `help:"Server log level." default:"info" enum:"debug,info,warn,error" env:"DORAS_LOG_LEVEL"`
	ShutdownTimout       uint   `help:"Graceful shutdown timeout (in seconds)." default:"20" env:"DORAS_SHUTDOWN_TIMEOUT"`
	InsecureAllowHTTP    bool   `help:"Allow INSECURE HTTP connections." default:"false" env:"DORAS_INSECURE_ALLOW_HTTP"`
	RequireClientAuth    bool   `help:"Always require clients to provide an authentication token, regardless of repo access rights." default:"true" env:"DORAS_REQUIRE_CLIENT_AUTH"`
	ExampleConfig        struct {
		Output string `help:"Write example config to this location instead of printing to stdout." type:"path"`
	} `cmd:"" help:"Print or store example config."`
	Run struct {
	} `cmd:"" help:"Run the server." default:"1"`
	Version bool `help:"Print version number version and exit." default:"false"`
}

// ServerConfigFile is used to parse the config files that can be used for more extensive configuration.
type ServerConfigFile struct {
	TrustedProxies []string             `yaml:"trusted-proxies"`
	Registries     map[string]RegConfig `yaml:"registries"`
}

// RegConfig stores the configuration for an OCI registry.
// Currently only wraps Auth, but more options may be added in the future.
type RegConfig struct {
	Auth RepoAuth `yaml:"auth"`
}

// RepoAuth stores registry login secrets.
// AccessToken is mutually exclusive to the other fields.
type RepoAuth struct {
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	AccessToken string `yaml:"access-token"`
}
