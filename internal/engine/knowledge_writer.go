package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// KnowledgeWriter streams knowledge pages to a structured directory.
type KnowledgeWriter struct {
	dir string
}

// NewKnowledgeWriter creates a writer for the given output directory.
func NewKnowledgeWriter(dir string) *KnowledgeWriter {
	return &KnowledgeWriter{dir: dir}
}

// Init creates the directory structure.
func (w *KnowledgeWriter) Init() error {
	dirs := []string{"pages", "screenshots", "har", "snapshots"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(w.dir, d), 0o755); err != nil {
			return fmt.Errorf("scout: knowledge writer: mkdir %s: %w", d, err)
		}
	}

	return nil
}

// WritePage writes a single page's data to the directory structure.
func (w *KnowledgeWriter) WritePage(kp *KnowledgePage) error {
	slug := urlToSlug(kp.URL)

	if kp.Markdown != "" {
		path := filepath.Join(w.dir, "pages", slug+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		if err := os.WriteFile(path, []byte(kp.Markdown), 0o644); err != nil {
			return fmt.Errorf("scout: knowledge writer: write markdown: %w", err)
		}
	}

	if kp.Screenshot != "" {
		data, err := base64.StdEncoding.DecodeString(kp.Screenshot)
		if err == nil {
			path := filepath.Join(w.dir, "screenshots", slug+".png")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}

			_ = os.WriteFile(path, data, 0o644)
		}
	}

	if len(kp.HAR) > 0 {
		path := filepath.Join(w.dir, "har", slug+".har")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		_ = os.WriteFile(path, kp.HAR, 0o644)
	}

	if kp.Snapshot != "" {
		path := filepath.Join(w.dir, "snapshots", slug+".yaml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		_ = os.WriteFile(path, []byte(kp.Snapshot), 0o644)
	}

	if len(kp.PDF) > 0 {
		path := filepath.Join(w.dir, "pages", slug+".pdf")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		_ = os.WriteFile(path, kp.PDF, 0o644)
	}

	return nil
}

// WriteManifest writes the KnowledgeResult (minus page content) as manifest.json.
func (w *KnowledgeWriter) WriteManifest(result *KnowledgeResult) error {
	manifest := *result
	manifest.Pages = nil

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: knowledge writer: marshal manifest: %w", err)
	}

	return os.WriteFile(filepath.Join(w.dir, "manifest.json"), data, 0o644)
}

// urlToSlug converts a URL to a filesystem-safe slug.
func urlToSlug(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "page"
	}

	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		return "index"
	}

	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = sanitizeFilename(p)
	}

	return filepath.Join(parts...)
}

// sanitizeFilename removes characters unsafe for filenames.
func sanitizeFilename(s string) string {
	replacer := strings.NewReplacer(
		"<", "", ">", "", ":", "", "\"", "",
		"|", "", "?", "", "*", "", "\\", "",
	)

	s = replacer.Replace(s)
	if s == "" {
		return "page"
	}

	return s
}
