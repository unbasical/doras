package configs

type ServerConfig struct {
	ConfigFile ServerConfigFile
	CliOpts    CLI
}

type CLI struct {
	HTTPPort             uint16 `help:"HTTP port to listen on." default:"8080" env:"DORAS_HTTP_PORT"`
	Host                 string `help:"Hostname to listen on." default:"127.0.0.1" env:"DORAS_HOST"`
	ConfigFilePath       string `help:"Path to the Doras server config file." env:"DORAS_CONFIG_FILE_PATH"`
	DockerConfigFilePath string `help:"Path to the docker config file which is used to access registry credentials." default:"~/.docker/config.json" env:"DOCKER_CONFIG_FILE_PATH"`
	LogLevel             string `help:"Server log level." default:"info" enum:"debug,info,warn,error" env:"DORAS_LOG_LEVEL"`
}

type ServerConfigFile struct {
	Sources        map[string]OrasSourceConfiguration `yaml:"sources"`
	Storage        StorageConfiguration               `yaml:"storage"`
	TrustedProxies []string                           `yaml:"trusted-proxies"`
}

type StorageConfiguration struct {
	Kind       string `yaml:"kind"`
	URL        string `yaml:"url"`
	EnableHTTP bool   `yaml:"enable-http"`
}

type OrasSourceConfiguration struct {
	EnableHTTP bool `yaml:"enable-http"`
}
