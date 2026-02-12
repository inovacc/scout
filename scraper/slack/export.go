package slack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/scout/scraper"
)

// ExportToJSON writes any data as indented JSON to the given file path.
func ExportToJSON(data any, path string) error {
	return scraper.ExportJSON(data, path)
}

// SaveCredentials persists credentials to a JSON file.
func SaveCredentials(creds scraper.Credentials, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("slack: create directory: %w", err)
	}

	b, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("slack: marshal credentials: %w", err)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("slack: write credentials: %w", err)
	}

	return nil
}

// LoadCredentials reads credentials from a JSON file.
func LoadCredentials(path string) (scraper.Credentials, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return scraper.Credentials{}, fmt.Errorf("slack: read credentials: %w", err)
	}

	var creds scraper.Credentials
	if err := json.Unmarshal(b, &creds); err != nil {
		return scraper.Credentials{}, fmt.Errorf("slack: parse credentials: %w", err)
	}

	return creds, nil
}
