package configs

type DorasServerConfig struct {
	Sources map[string]OrasSourceConfiguration `yaml:"sources"`
	Storage StorageConfiguration               `yaml:"storage"`
}

type StorageConfiguration struct {
	Kind       string `yaml:"kind"`
	URL        string `yaml:"url"`
	EnableHTTP bool   `yaml:"enable-http"`
}

type OrasSourceConfiguration struct {
	EnableHTTP bool `yaml:"enable-http"`
}
