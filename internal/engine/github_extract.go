package engine

import (
	"fmt"
)

// GitHubExtractRepo holds extracted repository metadata (extended).
type GitHubExtractRepo struct {
	Owner       string   `json:"owner"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Stars       int      `json:"stars"`
	Forks       int      `json:"forks"`
	Language    string   `json:"language"`
	Topics      []string `json:"topics"`
	License     string   `json:"license"`
	LastUpdated string   `json:"last_updated"`
	ReadmeHTML  string   `json:"readme_html,omitempty"`
}

// GitHubExtractIssue holds extracted issue/PR data with unified type.
type GitHubExtractIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"` // open, closed, merged
	Author    string   `json:"author"`
	Labels    []string `json:"labels"`
	CreatedAt string   `json:"created_at"`
	Body      string   `json:"body,omitempty"`
	Comments  int      `json:"comments"`
	IsPR      bool     `json:"is_pr"`
}

// GitHubExtractRelease holds extracted release data.
type GitHubExtractRelease struct {
	Tag         string   `json:"tag"`
	Name        string   `json:"name"`
	PublishedAt string   `json:"published_at"`
	Author      string   `json:"author"`
	Body        string   `json:"body,omitempty"`
	Assets      []string `json:"assets,omitempty"`
}

// GitHubExtractOption configures extraction.
type GitHubExtractOption func(*githubExtractOpts)

type githubExtractOpts struct {
	includeBody   bool
	includeReadme bool
	maxItems      int
	state         string // "open", "closed", "all"
	baseURL       string // for testing against local server
}

func githubExtractDefaults() *githubExtractOpts {
	return &githubExtractOpts{
		maxItems: 25,
		state:    "open",
	}
}

// WithGitHubExtractBody returns an option to include issue/PR/release body text.
func WithGitHubExtractBody() GitHubExtractOption {
	return func(o *githubExtractOpts) { o.includeBody = true }
}

// WithGitHubReadme returns an option to include README HTML for repo extraction.
func WithGitHubReadme() GitHubExtractOption {
	return func(o *githubExtractOpts) { o.includeReadme = true }
}

// WithGitHubExtractMaxItems limits items returned. Default: 25.
func WithGitHubExtractMaxItems(n int) GitHubExtractOption {
	return func(o *githubExtractOpts) {
		if n > 0 {
			o.maxItems = n
		}
	}
}

// WithGitHubExtractState filters by state for issues/PRs: "open", "closed", "all". Default: "open".
func WithGitHubExtractState(state string) GitHubExtractOption {
	return func(o *githubExtractOpts) { o.state = state }
}

// withGitHubExtractBaseURL overrides the base URL (for testing against a local server).
func withGitHubExtractBaseURL(baseURL string) GitHubExtractOption {
	return func(o *githubExtractOpts) { o.baseURL = baseURL }
}

// GitHubExtractRepoInfo navigates to github.com/{owner}/{repo} and extracts metadata.
func (b *Browser) GitHubExtractRepoInfo(owner, repo string, opts ...GitHubExtractOption) (*GitHubExtractRepo, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubExtractDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	baseHost := "https://github.com"
	if cfg.baseURL != "" {
		baseHost = cfg.baseURL
	}

	repoURL := fmt.Sprintf("%s/%s/%s", baseHost, owner, repo)

	page, err := b.NewPage(repoURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate repo: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	result, err := page.Eval(`() => {
		const repo = {};

		// Description
		const descEl = document.querySelector('[data-testid="about-description"], .f4.my-3, p.f4.my-3');
		repo.description = descEl ? descEl.textContent.trim() : '';

		// Stars
		const starsEl = document.querySelector('#repo-stars-counter-star, a[href$="/stargazers"] span, [id="repo-stars-counter-star"]');
		repo.stars = 0;
		if (starsEl) {
			const t = starsEl.textContent.trim().replace(/,/g, '');
			if (t.endsWith('k')) {
				repo.stars = Math.round(parseFloat(t) * 1000);
			} else {
				repo.stars = parseInt(t, 10) || 0;
			}
		}

		// Forks
		const forksEl = document.querySelector('#repo-network-counter, a[href$="/forks"] span, [id="repo-network-counter"]');
		repo.forks = 0;
		if (forksEl) {
			const t = forksEl.textContent.trim().replace(/,/g, '');
			if (t.endsWith('k')) {
				repo.forks = Math.round(parseFloat(t) * 1000);
			} else {
				repo.forks = parseInt(t, 10) || 0;
			}
		}

		// Language
		const langEl = document.querySelector('[data-ga-click*="language"], .BorderGrid-cell .h4, span[itemprop="programmingLanguage"]');
		repo.language = langEl ? langEl.textContent.trim() : '';

		// Topics
		repo.topics = [];
		document.querySelectorAll('a.topic-tag, a[data-octo-click="topic_click"]').forEach(el => {
			const t = el.textContent.trim();
			if (t) repo.topics.push(t);
		});

		// License
		const licenseEl = document.querySelector('a[data-analytics-event*="LICENSE"], a[href*="/blob/"] svg.octicon-law');
		if (licenseEl) {
			const parent = licenseEl.closest('a') || licenseEl;
			repo.license = parent.textContent.trim();
		} else {
			repo.license = '';
		}

		// Last updated
		const timeEl = document.querySelector('relative-time, [datetime]');
		repo.last_updated = timeEl ? (timeEl.getAttribute('datetime') || timeEl.textContent.trim()) : '';

		return repo;
	}`)
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval repo: %w", err)
	}

	r := &GitHubExtractRepo{
		Owner: owner,
		Name:  repo,
	}

	if m, ok := result.Value.(map[string]any); ok {
		if v, ok := m["description"].(string); ok {
			r.Description = v
		}

		if v, ok := m["stars"].(float64); ok {
			r.Stars = int(v)
		}

		if v, ok := m["forks"].(float64); ok {
			r.Forks = int(v)
		}

		if v, ok := m["language"].(string); ok {
			r.Language = v
		}

		if v, ok := m["license"].(string); ok {
			r.License = v
		}

		if v, ok := m["last_updated"].(string); ok {
			r.LastUpdated = v
		}

		if topics, ok := m["topics"].([]any); ok {
			for _, t := range topics {
				if s, ok := t.(string); ok {
					r.Topics = append(r.Topics, s)
				}
			}
		}
	}

	// Optionally fetch README HTML
	if cfg.includeReadme {
		readmeResult, readmeErr := page.Eval(`() => {
			const readme = document.querySelector('#readme .markdown-body, article.markdown-body');
			return readme ? readme.innerHTML : '';
		}`)
		if readmeErr == nil {
			if s, ok := readmeResult.Value.(string); ok {
				r.ReadmeHTML = s
			}
		}
	}

	return r, nil
}

// GitHubExtractIssues extracts the issue list from the issues tab.
func (b *Browser) GitHubExtractIssues(owner, repo string, opts ...GitHubExtractOption) ([]GitHubExtractIssue, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubExtractDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	baseHost := "https://github.com"
	if cfg.baseURL != "" {
		baseHost = cfg.baseURL
	}

	stateQuery := "is%3Aopen"

	switch cfg.state {
	case "closed":
		stateQuery = "is%3Aclosed"
	case "all":
		stateQuery = ""
	}

	issuesURL := fmt.Sprintf("%s/%s/%s/issues?q=%s", baseHost, owner, repo, stateQuery)

	page, err := b.NewPage(issuesURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate issues: %w", err)
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

			// Comments count
			const commentEl = row.querySelector('a[aria-label*="comment"], .Link--muted');
			issue.comments = 0;
			if (commentEl) {
				const ct = commentEl.textContent.trim().replace(/,/g, '');
				issue.comments = parseInt(ct, 10) || 0;
			}

			issue.is_pr = false;
			items.push(issue);
		}
		return items;
	}`, cfg.maxItems))
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval issues: %w", err)
	}

	issues := parseGitHubExtractIssues(result, false)

	if cfg.includeBody {
		for i := range issues {
			if issues[i].Number == 0 {
				continue
			}

			issueURL := fmt.Sprintf("%s/%s/%s/issues/%d", baseHost, owner, repo, issues[i].Number)

			body, bodyErr := b.fetchGitHubBody(issueURL)
			if bodyErr == nil {
				issues[i].Body = body
			}
		}
	}

	return issues, nil
}

// GitHubExtractPRs extracts the pull request list from the pulls tab.
func (b *Browser) GitHubExtractPRs(owner, repo string, opts ...GitHubExtractOption) ([]GitHubExtractIssue, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubExtractDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	baseHost := "https://github.com"
	if cfg.baseURL != "" {
		baseHost = cfg.baseURL
	}

	stateQuery := "is%3Aopen"

	switch cfg.state {
	case "closed":
		stateQuery = "is%3Aclosed"
	case "all":
		stateQuery = ""
	}

	prsURL := fmt.Sprintf("%s/%s/%s/pulls?q=%s", baseHost, owner, repo, stateQuery)

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

			// Comments count
			const commentEl = row.querySelector('a[aria-label*="comment"], .Link--muted');
			pr.comments = 0;
			if (commentEl) {
				const ct = commentEl.textContent.trim().replace(/,/g, '');
				pr.comments = parseInt(ct, 10) || 0;
			}

			pr.is_pr = true;
			items.push(pr);
		}
		return items;
	}`, cfg.maxItems))
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval prs: %w", err)
	}

	prs := parseGitHubExtractIssues(result, true)

	if cfg.includeBody {
		for i := range prs {
			if prs[i].Number == 0 {
				continue
			}

			prURL := fmt.Sprintf("%s/%s/%s/pull/%d", baseHost, owner, repo, prs[i].Number)

			body, bodyErr := b.fetchGitHubBody(prURL)
			if bodyErr == nil {
				prs[i].Body = body
			}
		}
	}

	return prs, nil
}

// GitHubExtractReleases extracts the releases list from the releases page.
func (b *Browser) GitHubExtractReleases(owner, repo string, opts ...GitHubExtractOption) ([]GitHubExtractRelease, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubExtractDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	baseHost := "https://github.com"
	if cfg.baseURL != "" {
		baseHost = cfg.baseURL
	}

	releasesURL := fmt.Sprintf("%s/%s/%s/releases", baseHost, owner, repo)

	page, err := b.NewPage(releasesURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate releases: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	includeBody := cfg.includeBody

	result, err := page.Eval(fmt.Sprintf(`(includeBody) => {
		const items = [];
		const sections = document.querySelectorAll('[data-testid="release-card"], .release, section[aria-label]');
		const max = %d;

		for (let i = 0; i < sections.length && i < max; i++) {
			const sec = sections[i];
			const rel = {};

			// Tag
			const tagEl = sec.querySelector('a[href*="/releases/tag/"] span, a[href*="/releases/tag/"]');
			rel.tag = tagEl ? tagEl.textContent.trim() : '';

			// Name
			const nameEl = sec.querySelector('h2 a, [data-testid="release-title"] a');
			rel.name = nameEl ? nameEl.textContent.trim() : rel.tag;

			// Published date
			const timeEl = sec.querySelector('relative-time, time');
			rel.published_at = timeEl ? (timeEl.getAttribute('datetime') || timeEl.textContent.trim()) : '';

			// Author
			const authorEl = sec.querySelector('a[data-hovercard-type="user"], .release-header a[href^="/"]');
			rel.author = authorEl ? authorEl.textContent.trim() : '';

			// Assets
			rel.assets = [];
			sec.querySelectorAll('a[href*="/releases/download/"]').forEach(a => {
				const name = a.textContent.trim();
				if (name) rel.assets.push(name);
			});

			// Body
			if (includeBody) {
				const bodyEl = sec.querySelector('.markdown-body');
				rel.body = bodyEl ? bodyEl.textContent.trim() : '';
			} else {
				rel.body = '';
			}

			items.push(rel);
		}
		return items;
	}`, cfg.maxItems), includeBody)
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval releases: %w", err)
	}

	var releases []GitHubExtractRelease

	if arr, ok := result.Value.([]any); ok {
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}

			rel := GitHubExtractRelease{}
			if v, ok := m["tag"].(string); ok {
				rel.Tag = v
			}

			if v, ok := m["name"].(string); ok {
				rel.Name = v
			}

			if v, ok := m["published_at"].(string); ok {
				rel.PublishedAt = v
			}

			if v, ok := m["author"].(string); ok {
				rel.Author = v
			}

			if v, ok := m["body"].(string); ok && v != "" {
				rel.Body = v
			}

			if assets, ok := m["assets"].([]any); ok {
				for _, a := range assets {
					if s, ok := a.(string); ok {
						rel.Assets = append(rel.Assets, s)
					}
				}
			}

			releases = append(releases, rel)
		}
	}

	return releases, nil
}

// parseGitHubExtractIssues converts an EvalResult into a slice of GitHubExtractIssue.
func parseGitHubExtractIssues(result *EvalResult, isPR bool) []GitHubExtractIssue {
	var issues []GitHubExtractIssue

	arr, ok := result.Value.([]any)
	if !ok {
		return nil
	}

	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		issue := GitHubExtractIssue{IsPR: isPR}
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

		if v, ok := m["comments"].(float64); ok {
			issue.Comments = int(v)
		}

		if v, ok := m["is_pr"].(bool); ok {
			issue.IsPR = v
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
