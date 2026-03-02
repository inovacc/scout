package runbook

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
)

// InteractiveConfig holds the configuration for interactive runbook creation.
type InteractiveConfig struct {
	Browser *scout.Browser
	URL     string
	Writer  io.Writer
	Reader  io.Reader
}

// InteractiveCreate walks the user through step-by-step runbook creation.
func InteractiveCreate(cfg InteractiveConfig) (*Runbook, error) {
	if cfg.Browser == nil {
		return nil, fmt.Errorf("runbook: interactive: nil browser")
	}

	if cfg.URL == "" {
		return nil, fmt.Errorf("runbook: interactive: empty URL")
	}

	if cfg.Writer == nil {
		return nil, fmt.Errorf("runbook: interactive: nil writer")
	}

	if cfg.Reader == nil {
		return nil, fmt.Errorf("runbook: interactive: nil reader")
	}

	scanner := bufio.NewScanner(cfg.Reader)
	w := cfg.Writer

	// Step 1: Analyze the site.
	_, _ = fmt.Fprintf(w, "Analyzing %s...\n", cfg.URL)

	analysis, err := AnalyzeSite(context.Background(), cfg.Browser, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("runbook: interactive: analyze: %w", err)
	}

	// Step 2: Print detected info.
	_, _ = fmt.Fprintf(w, "Page type: %s\n", analysis.PageType)

	_, _ = fmt.Fprintf(w, "Containers found: %d\n", len(analysis.Containers))
	for i, c := range analysis.Containers {
		_, _ = fmt.Fprintf(w, "  [%d] %s (%d items, %d fields)\n", i, c.Selector, c.Count, len(c.Fields))
		for _, f := range c.Fields {
			sample := f.Sample
			if len(sample) > 50 {
				sample = sample[:50] + "..."
			}

			_, _ = fmt.Fprintf(w, "       - %s: %s (sample: %s)\n", f.Name, f.Selector, sample)
		}
	}

	if len(analysis.Containers) == 0 && len(analysis.Forms) == 0 {
		return nil, fmt.Errorf("runbook: interactive: no containers or forms detected on page")
	}

	// Step 3: Choose container.
	containerIdx := 0

	if len(analysis.Containers) > 1 {
		choice := prompt(w, scanner, "Choose container index", "0")
		if choice != "" && choice != "0" {
			n := 0

			for _, ch := range choice {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				}
			}

			if n >= 0 && n < len(analysis.Containers) {
				containerIdx = n
			}
		}
	}

	// Step 4: Choose fields.
	var selectedFields []string

	if len(analysis.Containers) > 0 {
		container := analysis.Containers[containerIdx]

		var fieldNames []string
		for _, f := range container.Fields {
			fieldNames = append(fieldNames, f.Name)
		}

		defaultFields := strings.Join(fieldNames, ",")
		_, _ = fmt.Fprintf(w, "Available fields: %s\n", defaultFields)

		choice := prompt(w, scanner, "Fields to include (comma-separated)", defaultFields)
		if choice != "" {
			for f := range strings.SplitSeq(choice, ",") {
				f = strings.TrimSpace(f)
				if f != "" {
					selectedFields = append(selectedFields, f)
				}
			}
		}

		if len(selectedFields) == 0 {
			selectedFields = fieldNames
		}
	}

	// Step 5: Pagination.
	var genOpts []GenerateOption

	if analysis.Pagination != nil {
		_, _ = fmt.Fprintf(w, "Pagination detected: %s (confidence: %d%%)\n",
			analysis.Pagination.Strategy, analysis.Pagination.Confidence)

		usePagination := prompt(w, scanner, "Enable pagination? (yes/no)", "yes")
		if strings.HasPrefix(strings.ToLower(usePagination), "y") {
			maxPagesStr := prompt(w, scanner, "Max pages", "5")
			maxPages := 5
			n := 0

			for _, ch := range maxPagesStr {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				}
			}

			if n > 0 {
				maxPages = n
			}

			genOpts = append(genOpts, WithGenerateMaxPages(maxPages))
		} else {
			genOpts = append(genOpts, WithGenerateMaxPages(1))
		}
	}

	// Step 6: Runbook name.
	defaultName := inferName(analysis)

	name := prompt(w, scanner, "Runbook name", defaultName)
	if name == "" {
		name = defaultName
	}

	// Generate runbook with selected fields.
	if len(selectedFields) > 0 {
		genOpts = append(genOpts, WithGenerateFields(selectedFields...))
	}

	r, err := GenerateRunbook(analysis, genOpts...)
	if err != nil {
		return nil, fmt.Errorf("runbook: interactive: generate: %w", err)
	}

	r.Name = name

	// Step 7: Score selectors and print warnings.
	scores := ScoreRunbookSelectors(r)
	for sname, s := range scores {
		if s.Tier == "fragile" {
			_, _ = fmt.Fprintf(w, "Warning: fragile selector for %s: %s (score: %.2f)\n",
				sname, s.Selector, s.Score)
		}
	}

	return r, nil
}

// prompt writes a question to w and reads a line from scanner.
// If the user provides empty input, defaultVal is returned.
func prompt(w io.Writer, r *bufio.Scanner, question, defaultVal string) string {
	if defaultVal != "" {
		_, _ = fmt.Fprintf(w, "%s [%s]: ", question, defaultVal)
	} else {
		_, _ = fmt.Fprintf(w, "%s: ", question)
	}

	if r.Scan() {
		text := strings.TrimSpace(r.Text())
		if text != "" {
			return text
		}
	}

	return defaultVal
}
