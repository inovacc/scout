package engine

import (
	"fmt"
	"time"
)

// GitHubIssues navigates to the issues page of a GitHub repo and extracts issue metadata.
func (b *Browser) GitHubIssues(owner, name string, opts ...GitHubOption) ([]GitHubIssue, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	stateQuery := "is%3Aopen"

	switch cfg.state {
	case "closed":
		stateQuery = "is%3Aclosed"
	case "all":
		stateQuery = ""
	}

	baseHost := "https://github.com"
	if cfg.baseURL != "" {
		baseHost = cfg.baseURL
	}

	var issues []GitHubIssue

	for pageNum := 1; pageNum <= cfg.maxPages; pageNum++ {
		issuesURL := fmt.Sprintf("%s/%s/%s/issues?q=%s&page=%d", baseHost, owner, name, stateQuery, pageNum)

		if pageNum > 1 {
			time.Sleep(1 * time.Second)
		}

		page, err := b.NewPage(issuesURL)
		if err != nil {
			return nil, fmt.Errorf("scout: github: navigate issues: %w", err)
		}

		if err := page.WaitLoad(); err != nil {
			_ = page.Close()
			return nil, fmt.Errorf("scout: github: wait load: %w", err)
		}

		remaining := cfg.maxItems - len(issues)
		if remaining <= 0 {
			_ = page.Close()
			break
		}

		result, err := page.Eval(fmt.Sprintf(`() => {
			const items = [];
			const rows = document.querySelectorAll('[data-testid="issue-row"], .js-issue-row, [id^="issue_"]');
			const max = %d;

			for (let i = 0; i < rows.length && i < max; i++) {
				const row = rows[i];
				const issue = {};

				const titleLink = row.querySelector('a[data-hovercard-type="issue"], a[id^="issue_"]');
				if (titleLink) {
					issue.title = titleLink.textContent.trim();
					const href = titleLink.getAttribute('href') || '';
					const match = href.match(/\/issues\/(\d+)/);
					issue.number = match ? parseInt(match[1], 10) : 0;
				} else {
					issue.title = '';
					issue.number = 0;
				}

				const closedIcon = row.querySelector('.octicon-issue-closed, [data-testid="issue-closed-icon"]');
				issue.state = closedIcon ? 'closed' : 'open';

				const authorEl = row.querySelector('.opened-by a, a[data-hovercard-type="user"]');
				issue.author = authorEl ? authorEl.textContent.trim() : '';

				issue.labels = [];
				row.querySelectorAll('a[data-name], .IssueLabel, a.label').forEach(lbl => {
					const t = lbl.textContent.trim();
					if (t) issue.labels.push(t);
				});

				const timeEl = row.querySelector('relative-time, time');
				issue.created_at = timeEl ? (timeEl.getAttribute('datetime') || timeEl.textContent.trim()) : '';

				items.push(issue);
			}
			return items;
		}`, remaining))
		_ = page.Close()

		if err != nil {
			return nil, fmt.Errorf("scout: github: eval issues: %w", err)
		}

		pageIssues := parseGitHubIssues(result)
		if len(pageIssues) == 0 {
			break
		}

		issues = append(issues, pageIssues...)
	}

	// Fetch bodies if requested
	if cfg.includeBody {
		baseHost := "https://github.com"
		if cfg.baseURL != "" {
			baseHost = cfg.baseURL
		}

		for i := range issues {
			if issues[i].Number == 0 {
				continue
			}

			issueURL := fmt.Sprintf("%s/%s/%s/issues/%d", baseHost, owner, name, issues[i].Number)

			body, bodyErr := b.fetchGitHubBody(issueURL)
			if bodyErr == nil {
				issues[i].Body = body
			}
		}
	}

	return issues, nil
}

// GitHubPRs navigates to the pull requests page of a GitHub repo and extracts PR metadata.
func (b *Browser) GitHubPRs(owner, name string, opts ...GitHubOption) ([]GitHubPR, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	stateQuery := "is%3Aopen"

	switch cfg.state {
	case "closed":
		stateQuery = "is%3Aclosed"
	case "all":
		stateQuery = ""
	}

	prsURL := fmt.Sprintf("https://github.com/%s/%s/pulls?q=%s", owner, name, stateQuery)

	page, err := b.NewPage(prsURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate prs: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	result, err := page.Eval(fmt.Sprintf(`() => {
		const items = [];
		const rows = document.querySelectorAll('[data-testid="issue-row"], .js-issue-row, [id^="issue_"]');
		const max = %d;

		for (let i = 0; i < rows.length && i < max; i++) {
			const row = rows[i];
			const pr = {};

			const titleLink = row.querySelector('a[data-hovercard-type="pull_request"], a[id^="issue_"]');
			if (titleLink) {
				pr.title = titleLink.textContent.trim();
				const href = titleLink.getAttribute('href') || '';
				const match = href.match(/\/pull\/(\d+)/);
				pr.number = match ? parseInt(match[1], 10) : 0;
			} else {
				pr.title = '';
				pr.number = 0;
			}

			const mergedIcon = row.querySelector('.octicon-git-merge, [data-testid="pr-merged-icon"]');
			const closedIcon = row.querySelector('.octicon-git-pull-request-closed, [data-testid="pr-closed-icon"]');
			if (mergedIcon) {
				pr.state = 'merged';
			} else if (closedIcon) {
				pr.state = 'closed';
			} else {
				pr.state = 'open';
			}

			const authorEl = row.querySelector('.opened-by a, a[data-hovercard-type="user"]');
			pr.author = authorEl ? authorEl.textContent.trim() : '';

			pr.labels = [];
			row.querySelectorAll('a[data-name], .IssueLabel, a.label').forEach(lbl => {
				const t = lbl.textContent.trim();
				if (t) pr.labels.push(t);
			});

			const timeEl = row.querySelector('relative-time, time');
			pr.created_at = timeEl ? (timeEl.getAttribute('datetime') || timeEl.textContent.trim()) : '';

			items.push(pr);
		}
		return items;
	}`, cfg.maxItems))
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval prs: %w", err)
	}

	var prs []GitHubPR

	if arr, ok := result.Value.([]any); ok {
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}

			pr := GitHubPR{}
			if v, ok := m["number"].(float64); ok {
				pr.Number = int(v)
			}

			if v, ok := m["title"].(string); ok {
				pr.Title = v
			}

			if v, ok := m["state"].(string); ok {
				pr.State = v
			}

			if v, ok := m["author"].(string); ok {
				pr.Author = v
			}

			if v, ok := m["created_at"].(string); ok {
				pr.CreatedAt = v
			}

			if labels, ok := m["labels"].([]any); ok {
				for _, l := range labels {
					if s, ok := l.(string); ok {
						pr.Labels = append(pr.Labels, s)
					}
				}
			}

			prs = append(prs, pr)
		}
	}

	if cfg.includeBody {
		for i := range prs {
			if prs[i].Number == 0 {
				continue
			}

			prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, name, prs[i].Number)

			body, bodyErr := b.fetchGitHubBody(prURL)
			if bodyErr == nil {
				prs[i].Body = body
			}
		}
	}

	return prs, nil
}

// parseGitHubIssues converts an EvalResult into a slice of GitHubIssue.
func parseGitHubIssues(result *EvalResult) []GitHubIssue {
	var issues []GitHubIssue

	arr, ok := result.Value.([]any)
	if !ok {
		return nil
	}

	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		issue := GitHubIssue{}
		if v, ok := m["number"].(float64); ok {
			issue.Number = int(v)
		}

		if v, ok := m["title"].(string); ok {
			issue.Title = v
		}

		if v, ok := m["state"].(string); ok {
			issue.State = v
		}

		if v, ok := m["author"].(string); ok {
			issue.Author = v
		}

		if v, ok := m["created_at"].(string); ok {
			issue.CreatedAt = v
		}

		if labels, ok := m["labels"].([]any); ok {
			for _, l := range labels {
				if s, ok := l.(string); ok {
					issue.Labels = append(issue.Labels, s)
				}
			}
		}

		issues = append(issues, issue)
	}

	return issues
}

// fetchGitHubBody navigates to an issue or PR page and extracts the body as markdown.
func (b *Browser) fetchGitHubBody(pageURL string) (string, error) {
	page, err := b.NewPage(pageURL)
	if err != nil {
		return "", fmt.Errorf("scout: github: fetch body: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return "", fmt.Errorf("scout: github: wait load body: %w", err)
	}

	result, err := page.Eval(`() => {
		const body = document.querySelector('.js-comment-body, .comment-body, [data-testid="issue-body"]');
		return body ? body.textContent.trim() : '';
	}`)
	if err != nil {
		return "", fmt.Errorf("scout: github: eval body: %w", err)
	}

	if s, ok := result.Value.(string); ok {
		return s, nil
	}

	return "", nil
}
