package engine

import (
	"fmt"
	"net/url"
	"time"
)

// GitHubSearchCode searches GitHub code search and extracts results.
func (b *Browser) GitHubSearchCode(query string, opts ...GitHubOption) ([]GitHubCodeResult, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	// Scope to repo if specified
	if cfg.repoOwner != "" && cfg.repoName != "" {
		query = fmt.Sprintf("%s repo:%s/%s", query, cfg.repoOwner, cfg.repoName)
	}

	baseHost := "https://github.com"
	if cfg.baseURL != "" {
		baseHost = cfg.baseURL
	}

	var results []GitHubCodeResult

	for pageNum := 1; pageNum <= cfg.maxPages; pageNum++ {
		searchURL := fmt.Sprintf("%s/search?q=%s&type=code&p=%d", baseHost, url.QueryEscape(query), pageNum)

		if pageNum > 1 {
			time.Sleep(1 * time.Second)
		}

		page, err := b.NewPage(searchURL)
		if err != nil {
			return nil, fmt.Errorf("scout: github: navigate code search: %w", err)
		}

		if err := page.WaitLoad(); err != nil {
			_ = page.Close()
			return nil, fmt.Errorf("scout: github: wait load: %w", err)
		}

		remaining := cfg.maxItems - len(results)
		if remaining <= 0 {
			_ = page.Close()
			break
		}

		result, err := page.Eval(fmt.Sprintf(`() => {
			const items = [];
			const rows = document.querySelectorAll('.code-list-item, [data-testid="code-result"], .hx_hit-code');
			const max = %d;

			for (let i = 0; i < rows.length && i < max; i++) {
				const row = rows[i];
				const item = {};

				// Repo name
				const repoLink = row.querySelector('a[data-testid="code-result-repo"], .hx_hit-code-title a, a.text-bold');
				item.repo = repoLink ? repoLink.textContent.trim() : '';

				// File path
				const fileLink = row.querySelector('a[data-testid="code-result-path"], .hx_hit-code-title a:last-child, a[title]');
				item.file_path = fileLink ? (fileLink.getAttribute('title') || fileLink.textContent.trim()) : '';

				// Snippet
				const snippetEl = row.querySelector('.code-list-item-code, [data-testid="code-result-snippet"], .hx_hit-code .blob-code, .code-snippet');
				item.snippet = snippetEl ? snippetEl.textContent.trim() : '';

				items.push(item);
			}
			return items;
		}`, remaining))
		_ = page.Close()

		if err != nil {
			return nil, fmt.Errorf("scout: github: eval code search: %w", err)
		}

		pageResults := parseGitHubCodeResults(result)
		if len(pageResults) == 0 {
			break
		}

		results = append(results, pageResults...)
	}

	return results, nil
}

// parseGitHubCodeResults converts an EvalResult into a slice of GitHubCodeResult.
func parseGitHubCodeResults(result *EvalResult) []GitHubCodeResult {
	var results []GitHubCodeResult

	arr, ok := result.Value.([]any)
	if !ok {
		return nil
	}

	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		r := GitHubCodeResult{}
		if v, ok := m["repo"].(string); ok {
			r.Repo = v
		}

		if v, ok := m["file_path"].(string); ok {
			r.FilePath = v
		}

		if v, ok := m["snippet"].(string); ok {
			r.Snippet = v
		}

		results = append(results, r)
	}

	return results
}
