package bundle

type bundleConfiguration struct {
	Apps map[string]app `yaml:"apps" json:"apps"`
}

type app struct {
	AppName string `yaml:"appName,omitempty" json:"appName,omitempty"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
}
