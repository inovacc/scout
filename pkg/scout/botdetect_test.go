package scout

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// botCheckSite is a site that tests for bot/automation fingerprints.
type botCheckSite struct {
	Name string
	URL  string
	// Check returns true if bot was detected, along with a description.
	Check func(p *Page) (detected bool, detail string)
}

// botCheckSites lists public bot-detection test sites.
// These are legitimate services designed to test browser fingerprinting.
var botCheckSites = []botCheckSite{
	{
		Name: "bot.sannysoft.com",
		URL:  "https://bot.sannysoft.com/",
		Check: func(p *Page) (bool, string) {
			// This site shows a table of detection tests. "FAIL" cells mean bot detected.
			result, err := p.Eval(`() => {
				const rows = document.querySelectorAll('table tr');
				const fails = [];
				for (const row of rows) {
					const cells = row.querySelectorAll('td');
					if (cells.length >= 2) {
						const name = cells[0].textContent.trim();
						const val = cells[1].textContent.trim();
						const cls = cells[1].className || '';
						if (cls.includes('failed') || val === 'FAIL') {
							fails.push(name + ': ' + val);
						}
					}
				}
				return JSON.stringify(fails);
			}`)
			if err != nil {
				return false, fmt.Sprintf("eval error: %v", err)
			}
			s := result.String()
			if s == "[]" || s == "null" || s == "" {
				return false, "no failures detected"
			}
			return true, s
		},
	},
	{
		Name: "arh.antoinevastel.com/bots/areyouheadless",
		URL:  "https://arh.antoinevastel.com/bots/areyouheadless",
		Check: func(p *Page) (bool, string) {
			result, err := p.Eval(`() => document.body.innerText`)
			if err != nil {
				return false, fmt.Sprintf("eval error: %v", err)
			}
			text := result.String()
			lower := strings.ToLower(text)
			// The page says "You are not Chrome headless" when passing,
			// or "You are Chrome headless" when detected.
			if strings.Contains(lower, "you are not chrome headless") {
				return false, "passed: not detected as headless"
			}
			if strings.Contains(lower, "you are chrome headless") ||
				strings.Contains(lower, "you are a bot") {
				return true, "detected as headless/bot"
			}
			return false, text
		},
	},
	{
		Name: "infosimples/detect-headless",
		URL:  "https://infosimples.github.io/detect-headless/",
		Check: func(p *Page) (bool, string) {
			result, err := p.Eval(`() => {
				const results = [];
				document.querySelectorAll('.test-result').forEach(el => {
					const label = el.closest('tr')?.querySelector('td:first-child')?.textContent?.trim() || '';
					const value = el.textContent.trim();
					if (value.toLowerCase().includes('headless') || value.toLowerCase().includes('bot')) {
						results.push(label + ': ' + value);
					}
				});
				document.querySelectorAll('.failed, .red, [style*="red"]').forEach(el => {
					results.push('FAILED: ' + el.textContent.trim().substring(0, 80));
				});
				return JSON.stringify(results);
			}`)
			if err != nil {
				return false, fmt.Sprintf("eval error: %v", err)
			}
			s := result.String()
			if s == "[]" || s == "null" || s == "" {
				return false, "no bot indicators"
			}
			return true, s
		},
	},
	{
		Name: "pixelscan.net",
		URL:  "https://pixelscan.net/",
		Check: func(p *Page) (bool, string) {
			// Pixelscan shows a threat/consistency score and flags inconsistencies.
			result, err := p.Eval(`() => {
				const text = document.body.innerText.toLowerCase();
				const flags = [];
				if (text.includes('inconsistent')) flags.push('inconsistent fingerprint');
				if (text.includes('bot detected') || text.includes('automation')) flags.push('bot/automation detected');
				if (text.includes('threat')) {
					const m = text.match(/threat[:\s]*(high|medium)/i);
					if (m) flags.push('threat level: ' + m[1]);
				}
				// Look for red warning elements
				const warns = document.querySelectorAll('[class*="warning"], [class*="danger"], [class*="red"], [class*="fail"]');
				warns.forEach(el => {
					const t = el.textContent.trim().substring(0, 60);
					if (t) flags.push(t);
				});
				return JSON.stringify(flags);
			}`)
			if err != nil {
				return false, fmt.Sprintf("eval error: %v", err)
			}
			s := result.String()
			if s == "[]" || s == "null" || s == "" {
				return false, "no issues detected"
			}
			return true, s
		},
	},
	{
		Name: "brotector",
		URL:  "https://seleniumbase.github.io/apps/brotector",
		Check: func(p *Page) (bool, string) {
			// Brotector shows pass/fail for various bot detection checks.
			result, err := p.Eval(`() => {
				const text = document.body.innerText;
				const lower = text.toLowerCase();
				if (lower.includes('bot detected') || lower.includes('failed')) {
					const lines = text.split('\n').filter(l =>
						l.toLowerCase().includes('fail') || l.toLowerCase().includes('bot detected')
					).map(l => l.trim()).filter(l => l.length > 0 && l.length < 100);
					return JSON.stringify(lines);
				}
				if (lower.includes('passed') || lower.includes('human')) {
					return '';
				}
				return '';
			}`)
			if err != nil {
				return false, fmt.Sprintf("eval error: %v", err)
			}
			s := result.String()
			if s == "" || s == "[]" || s == "null" {
				return false, "no bot indicators"
			}
			return true, s
		},
	},
	{
		Name: "creepjs",
		URL:  "https://abrahamjuliot.github.io/creepjs/",
		Check: func(p *Page) (bool, string) {
			// CreepJS computes a "trust score" and flags lies/fingerprint anomalies.
			result, err := p.Eval(`() => {
				const text = document.body.innerText.toLowerCase();
				const flags = [];
				// Look for lie or bot indicators
				if (text.includes('lie detected') || text.includes('lies detected')) flags.push('lies detected');
				if (text.includes('bot') && !text.includes('about')) flags.push('bot indicator found');
				// Check trust score — lower is worse
				const m = text.match(/trust\s*score[:\s]*([0-9.]+%?)/i);
				if (m) {
					flags.push('trust score: ' + m[1]);
					const score = parseFloat(m[1]);
					if (!isNaN(score) && score < 50) flags.push('low trust score');
				}
				// Check for headless markers
				const headless = document.querySelectorAll('[class*="headless"], [class*="bot"], [class*="lie"]');
				headless.forEach(el => {
					const t = el.textContent.trim().substring(0, 80);
					if (t && t.length > 2) flags.push(t);
				});
				return JSON.stringify(flags);
			}`)
			if err != nil {
				return false, fmt.Sprintf("eval error: %v", err)
			}
			s := result.String()
			if s == "[]" || s == "null" || s == "" {
				return false, "no issues detected"
			}
			return true, s
		},
	},
	{
		Name: "fingerprint.com/playground",
		URL:  "https://demo.fingerprint.com/playground",
		Check: func(p *Page) (bool, string) {
			// Fingerprint.com detects bot/automation and shows a bot detection result.
			result, err := p.Eval(`() => {
				const text = document.body.innerText.toLowerCase();
				const flags = [];
				if (text.includes('bot detected') || text.includes('automation tool')) {
					flags.push('bot/automation detected');
				}
				if (text.includes('headless')) flags.push('headless detected');
				// Look for bot probability or confidence scores
				const m = text.match(/bot\s*(?:probability|confidence|score)[:\s]*([0-9.]+%?)/i);
				if (m) flags.push('bot score: ' + m[1]);
				return JSON.stringify(flags);
			}`)
			if err != nil {
				return false, fmt.Sprintf("eval error: %v", err)
			}
			s := result.String()
			if s == "[]" || s == "null" || s == "" {
				return false, "no bot indicators"
			}
			return true, s
		},
	},
}

// TestBotDetection_NoStealth visits bot-detection sites WITHOUT stealth mode.
// It keeps visiting until at least one site detects us as a bot.
// This test validates that bot detection sites DO catch an unprotected headless browser.
func TestBotDetection_NoStealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires network access")
	}

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTimeout(30*time.Second),
		WithoutBridge(),
	)
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	detected := false
	for _, site := range botCheckSites {
		t.Run(site.Name, func(t *testing.T) {
			page, err := b.NewPage(site.URL)
			if err != nil {
				t.Logf("skip %s: navigate error: %v", site.Name, err)
				return
			}
			defer func() { _ = page.Close() }()

			if err := page.WaitLoad(); err != nil {
				t.Logf("skip %s: wait load error: %v", site.Name, err)
				return
			}
			// Extra wait for JS-heavy pages
			page.WaitDOMStable(500*time.Millisecond, 0.1)

			isBot, detail := site.Check(page)
			if isBot {
				t.Logf("BOT DETECTED by %s: %s", site.Name, detail)
				detected = true
			} else {
				t.Logf("not detected by %s: %s", site.Name, detail)
			}
		})

		if detected {
			break
		}
	}

	if !detected {
		t.Log("WARNING: no site detected us as a bot (all sites may be down or changed)")
	}
}

// TestBotDetection_WithStealth visits the SAME bot-detection sites WITH stealth mode.
// Stealth mode should reduce or eliminate bot detection compared to the non-stealth test.
func TestBotDetection_WithStealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires network access")
	}

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTimeout(30*time.Second),
		WithStealth(),
		WithoutBridge(),
	)
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	detectedCount := 0
	for _, site := range botCheckSites {
		t.Run(site.Name, func(t *testing.T) {
			page, err := b.NewPage(site.URL)
			if err != nil {
				t.Logf("skip %s: navigate error: %v", site.Name, err)
				return
			}
			defer func() { _ = page.Close() }()

			if err := page.WaitLoad(); err != nil {
				t.Logf("skip %s: wait load error: %v", site.Name, err)
				return
			}
			page.WaitDOMStable(500*time.Millisecond, 0.1)

			isBot, detail := site.Check(page)
			if isBot {
				t.Logf("BOT DETECTED (even with stealth) by %s: %s", site.Name, detail)
				detectedCount++
			} else {
				t.Logf("PASSED (stealth worked) %s: %s", site.Name, detail)
			}
		})
	}

	t.Logf("stealth result: %d/%d sites detected bot", detectedCount, len(botCheckSites))
}
