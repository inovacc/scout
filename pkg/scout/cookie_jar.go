package scout

import (
	"encoding/json"
	"fmt"
	"os"
)

// SaveCookiesToFile exports the page's cookies to a JSON file.
// Only non-session cookies (with an expiry) are saved by default.
// Pass includeSession=true to include session cookies as well.
func (p *Page) SaveCookiesToFile(path string, includeSession bool) error {
	cookies, err := p.GetCookies()
	if err != nil {
		return fmt.Errorf("scout: save cookies: %w", err)
	}

	if !includeSession {
		filtered := cookies[:0]
		for _, c := range cookies {
			if !c.Expires.IsZero() {
				filtered = append(filtered, c)
			}
		}
		cookies = filtered
	}

	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: save cookies: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("scout: save cookies: write: %w", err)
	}

	return nil
}

// LoadCookiesFromFile reads cookies from a JSON file and sets them on the page.
func (p *Page) LoadCookiesFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("scout: load cookies: read: %w", err)
	}

	var cookies []Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return fmt.Errorf("scout: load cookies: unmarshal: %w", err)
	}

	if len(cookies) == 0 {
		return nil
	}

	return p.SetCookies(cookies...)
}
