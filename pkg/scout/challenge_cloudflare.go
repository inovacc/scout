package scout

import (
	"fmt"
	"time"
)

// solveCloudflareWait waits for Cloudflare's JS challenge to complete automatically.
// Many Cloudflare challenges resolve on their own once the JS runs; this solver
// polls for the cf_clearance cookie and checks if the challenge page has been replaced.
func solveCloudflareWait(page *Page, _ ChallengeInfo) error {
	if page == nil || page.page == nil {
		return fmt.Errorf("scout: challenge: cloudflare: nil page")
	}

	const (
		pollInterval = 500 * time.Millisecond
		maxWait      = 15 * time.Second
	)

	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		// Check for cf_clearance cookie.
		cookies, err := page.GetCookies()
		if err == nil {
			for _, c := range cookies {
				if c.Name == "cf_clearance" {
					// Cookie obtained — verify the challenge page is gone.
					has, _ := page.Has("#cf-browser-verification")
					if !has {
						return nil
					}
				}
			}
		}

		// Also check if the challenge elements have disappeared.
		hasCF, _ := page.Has("#cf-browser-verification")

		hasRunning, _ := page.Has("#challenge-running")
		if !hasCF && !hasRunning {
			// Challenge page replaced with content.
			return nil
		}

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("scout: challenge: cloudflare: timed out waiting for JS challenge (15s)")
}

// solveTurnstile attempts to solve a Cloudflare Turnstile widget by clicking
// the checkbox element within the widget iframe.
func solveTurnstile(page *Page, _ ChallengeInfo) error {
	if page == nil || page.page == nil {
		return fmt.Errorf("scout: challenge: turnstile: nil page")
	}

	const (
		pollInterval = 500 * time.Millisecond
		maxWait      = 10 * time.Second
	)

	// Try to find and click the Turnstile container.
	// Turnstile renders a widget; clicking the container area often triggers it.
	container, err := page.Element(".cf-turnstile")
	if err == nil && container != nil {
		_ = container.Click()
	}

	// Poll for completion: Turnstile sets a hidden input with the token.
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		result, evalErr := page.Eval(`(function() {
			var input = document.querySelector('input[name="cf-turnstile-response"]');
			if (input && input.value) return input.value;
			return "";
		})()`)
		if evalErr == nil && result.String() != "" {
			return nil
		}

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("scout: challenge: turnstile: timed out waiting for solution (10s)")
}

// persistClearanceCookies extracts cf_clearance and related Cloudflare cookies
// from the page for reuse in subsequent sessions.
func persistClearanceCookies(page *Page) ([]map[string]string, error) { //nolint:unused
	if page == nil || page.page == nil {
		return nil, fmt.Errorf("scout: challenge: persist cookies: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("scout: challenge: persist cookies: %w", err)
	}

	var result []map[string]string

	for _, c := range cookies {
		if c.Name == "cf_clearance" || c.Name == "__cf_bm" || c.Name == "cf_chl_2" {
			result = append(result, map[string]string{
				"name":   c.Name,
				"value":  c.Value,
				"domain": c.Domain,
				"path":   c.Path,
			})
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("scout: challenge: no clearance cookies found")
	}

	return result, nil
}
