package scraper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials holds authentication data extracted from a browser session.
type Credentials struct {
	Token   string            `json:"token"`
	Cookies map[string]string `json:"cookies,omitempty"`
	Extra   map[string]string `json:"extra,omitempty"`
}

// Progress reports the current state of a long-running scraper operation.
type Progress struct {
	Phase   string `json:"phase"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

// ProgressFunc is a callback for receiving progress updates.
type ProgressFunc func(Progress)

// AuthError indicates an authentication failure (invalid token, expired session, etc.).
type AuthError struct {
	Reason string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("scraper: auth: %s", e.Reason)
}

// RateLimitError indicates the remote service is throttling requests.
type RateLimitError struct {
	RetryAfter int // seconds to wait before retrying, 0 if unknown
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("scraper: rate limited: retry after %ds", e.RetryAfter)
	}

	return "scraper: rate limited"
}

// ExportJSON writes data as indented JSON to the given file path.
// It creates parent directories as needed.
func ExportJSON(data any, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("scraper: create directory: %w", err)
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("scraper: marshal json: %w", err)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("scraper: write file: %w", err)
	}

	return nil
}
