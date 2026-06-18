package b3pipe

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadSections reads and validates a sections.yaml file.
func LoadSections(path string) (*SectionsConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("b3: read sections: %w", err)
	}
	var cfg SectionsConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("b3: parse sections: %w", err)
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("b3: sections: base_url is required")
	}
	if len(cfg.Sections) == 0 {
		return nil, fmt.Errorf("b3: sections: at least one section is required")
	}
	for i, s := range cfg.Sections {
		if s.ID == "" || s.Endpoint == "" || s.Output == "" {
			return nil, fmt.Errorf("b3: sections[%d]: id, endpoint, output are required", i)
		}
	}
	return &cfg, nil
}
