package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontmatterOut is the serialisation shape for YAML frontmatter on Write.
// omitempty ensures only non-empty optional fields are emitted.
type frontmatterOut struct {
	Type        string   `yaml:"type"`
	Title       string   `yaml:"title,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Resource    string   `yaml:"resource,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
	Timestamp   string   `yaml:"timestamp,omitempty"`
}

// Write serialises every Concept in the Bundle to <dir>/<id>.md.
// Parent directories are created as needed (0o755). Files are written 0o644.
func (b *Bundle) Write(dir string) error {
	for _, c := range b.Concepts {
		if err := writeConcept(dir, c); err != nil {
			return fmt.Errorf("okf: write concept %q: %w", c.ID, err)
		}
	}
	return nil
}

func writeConcept(dir string, c Concept) error {
	// Build the file path: dir + id + ".md", using OS path separators.
	relPath := filepath.FromSlash(c.ID + ".md")
	fullPath := filepath.Join(dir, relPath)

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("okf: mkdir: %w", err)
	}

	// Marshal frontmatter.
	fm := frontmatterOut{
		Type:        c.Type,
		Title:       c.Title,
		Description: c.Description,
		Resource:    c.Resource,
		Tags:        c.Tags,
		Timestamp:   c.Timestamp,
	}

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return fmt.Errorf("okf: marshal frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n")
	if c.Body != "" {
		sb.WriteString(c.Body)
	}

	if err := os.WriteFile(fullPath, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("okf: write file: %w", err)
	}
	return nil
}
