package okf

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Bundle is a collection of Concepts read from or written to a directory tree.
type Bundle struct {
	Concepts []Concept
}

// frontmatterIn is the deserialisation shape for YAML frontmatter on Read.
// Unknown fields are silently ignored (forward-compat).
type frontmatterIn struct {
	Type        string   `yaml:"type"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Resource    string   `yaml:"resource"`
	Tags        []string `yaml:"tags"`
	Timestamp   string   `yaml:"timestamp"`
}

// Read walks dir for *.md files, parses frontmatter + body, and returns a Bundle.
// The concept ID is the file's path relative to dir, with the ".md" extension
// stripped and path separators normalised to forward slashes.
func Read(dir string) (*Bundle, error) {
	var concepts []Concept

	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("okf: walk: %w", err)
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}

		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return fmt.Errorf("okf: rel path: %w", err)
		}

		// Normalise to forward slashes and strip .md.
		id := strings.TrimSuffix(filepath.ToSlash(rel), ".md")

		c, err := parseFile(p, id)
		if err != nil {
			return fmt.Errorf("okf: parse %q: %w", rel, err)
		}
		concepts = append(concepts, c)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Bundle{Concepts: concepts}, nil
}

// parseFile reads and parses a single .md file into a Concept.
func parseFile(path, id string) (Concept, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Concept{}, fmt.Errorf("okf: read file: %w", err)
	}

	content := string(data)

	// Split frontmatter from body.
	fm, body := splitFrontmatter(content)

	c := Concept{
		ID:   id,
		Body: body,
	}

	if fm != "" {
		var parsed frontmatterIn
		if err := yaml.Unmarshal([]byte(fm), &parsed); err != nil {
			return Concept{}, fmt.Errorf("okf: parse frontmatter: %w", err)
		}
		c.Type = parsed.Type
		c.Title = parsed.Title
		c.Description = parsed.Description
		c.Resource = parsed.Resource
		c.Tags = parsed.Tags
		c.Timestamp = parsed.Timestamp
	}

	return c, nil
}

// splitFrontmatter splits content into (frontmatterYAML, body).
// Frontmatter is the text between the first two "---" fences.
// If no valid frontmatter block is found, returns ("", content).
func splitFrontmatter(content string) (fm, body string) {
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return "", content
	}

	// Find the closing "---" fence after the opening one.
	skip := 4
	if strings.HasPrefix(content, "---\r\n") {
		skip = 5
	}
	rest := content[skip:] // skip the leading "---\n" (4) or "---\r\n" (5)

	endIdx := strings.Index(rest, "\n---\n")
	if endIdx == -1 {
		// Try CRLF variant.
		endIdx = strings.Index(rest, "\r\n---\r\n")
		if endIdx == -1 {
			return "", content
		}
		fm = rest[:endIdx]
		body = rest[endIdx+7:] // len("\r\n---\r\n") = 7
		return fm, body
	}

	fm = rest[:endIdx]
	body = rest[endIdx+5:] // len("\n---\n") = 5
	return fm, body
}
