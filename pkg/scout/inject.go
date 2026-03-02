package scout

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// WithInjectJS loads JavaScript files that are injected into every new page
// via EvalOnNewDocument before any page scripts run.
func WithInjectJS(paths ...string) Option {
	return func(o *options) {
		for _, p := range paths {
			data, err := os.ReadFile(p)
			if err != nil {
				// Store the error as a sentinel script that will be caught at page creation.
				o.injectErr = fmt.Errorf("scout: inject: read %s: %w", p, err)
				return
			}

			if len(data) > 0 {
				o.injectScripts = append(o.injectScripts, string(data))
			}
		}
	}
}

// WithInjectDir loads all .js files from a directory for injection into every new page.
// Files are sorted alphabetically for deterministic injection order.
func WithInjectDir(dir string) Option {
	return func(o *options) {
		matches, err := filepath.Glob(filepath.Join(dir, "*.js"))
		if err != nil {
			o.injectErr = fmt.Errorf("scout: inject: glob %s: %w", dir, err)
			return
		}

		sort.Strings(matches)

		for _, m := range matches {
			data, err := os.ReadFile(m)
			if err != nil {
				o.injectErr = fmt.Errorf("scout: inject: read %s: %w", m, err)
				return
			}

			if len(data) > 0 {
				o.injectScripts = append(o.injectScripts, string(data))
			}
		}
	}
}

// WithInjectCode injects raw JavaScript code strings into every new page
// via EvalOnNewDocument before any page scripts run.
func WithInjectCode(code ...string) Option {
	return func(o *options) {
		for _, c := range code {
			if c != "" {
				o.injectScripts = append(o.injectScripts, c)
			}
		}
	}
}
