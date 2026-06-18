package b3pipe

type AuthConfig struct {
	Mode            string `yaml:"mode"`              // "cookie" or "bearer"
	TokenStorageKey string `yaml:"token_storage_key"` // localStorage key holding the access JWT (bearer mode)
}

type Section struct {
	ID         string `yaml:"id"`
	Endpoint   string `yaml:"endpoint"`
	Output     string `yaml:"output"`
	RecordPath string `yaml:"record_path"`
}

type SectionsConfig struct {
	BaseURL  string     `yaml:"base_url"`
	Auth     AuthConfig `yaml:"auth"`
	Sections []Section  `yaml:"sections"`
}
