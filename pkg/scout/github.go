package scout

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// GitHubRepo holds metadata about a GitHub repository.
type GitHubRepo struct {
	Owner       string   `json:"owner"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Stars       int      `json:"stars"`
	Forks       int      `json:"forks"`
	Language    string   `json:"language"`
	Topics      []string `json:"topics"`
	License     string   `json:"license"`
	ReadmeMD    string   `json:"readme_md,omitempty"`
}

// GitHubIssue holds metadata about a GitHub issue.
type GitHubIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Author    string   `json:"author"`
	Labels    []string `json:"labels"`
	Body      string   `json:"body,omitempty"`
	CreatedAt string   `json:"created_at"`
}

// GitHubPR holds metadata about a GitHub pull request.
type GitHubPR struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Author    string   `json:"author"`
	Labels    []string `json:"labels"`
	Body      string   `json:"body,omitempty"`
	CreatedAt string   `json:"created_at"`
}

// GitHubUser holds metadata about a GitHub user profile.
type GitHubUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Location    string `json:"location"`
	Repos       int    `json:"repos"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
}

// GitHubRelease holds metadata about a GitHub release.
type GitHubRelease struct {
	Tag    string `json:"tag"`
	Name   string `json:"name"`
	Body   string `json:"body"`
	Date   string `json:"date"`
	Assets int    `json:"assets"`
}

// GitHubCodeResult holds a single code search result from GitHub.
type GitHubCodeResult struct {
	Repo     string `json:"repo"`
	FilePath string `json:"file_path"`
	Snippet  string `json:"snippet"`
}

// GitHubOption configures GitHub extraction behavior.
type GitHubOption func(*githubConfig)

type githubConfig struct {
	includeBody bool
	maxItems    int
	maxPages    int
	state       string // "open", "closed", "all"
	repoOwner   string
	repoName    string
	baseURL     string // for testing against local server
}

func githubDefaults() *githubConfig {
	return &githubConfig{
		maxItems: 30,
		maxPages: 1,
		state:    "open",
	}
}

// WithGitHubBody includes the full body of issues and pull requests.
func WithGitHubBody() GitHubOption {
	return func(c *githubConfig) { c.includeBody = true }
}

// WithGitHubMaxItems limits the number of items returned. Default: 30.
func WithGitHubMaxItems(n int) GitHubOption {
	return func(c *githubConfig) { c.maxItems = n }
}

// WithGitHubState filters issues/PRs by state: "open", "closed", or "all". Default: "open".
func WithGitHubState(state string) GitHubOption {
	return func(c *githubConfig) { c.state = state }
}

// WithGitHubMaxPages sets the maximum number of pages to fetch for paginated results. Default: 1.
func WithGitHubMaxPages(n int) GitHubOption {
	return func(c *githubConfig) {
		if n > 0 {
			c.maxPages = n
		}
	}
}

// WithGitHubRepo scopes code search to a specific repository by appending repo:owner/name to the query.
func WithGitHubRepo(owner, repo string) GitHubOption {
	return func(c *githubConfig) {
		c.repoOwner = owner
		c.repoName = repo
	}
}

// withGitHubBaseURL overrides the base URL (for testing against a local server).
func withGitHubBaseURL(baseURL string) GitHubOption {
	return func(c *githubConfig) { c.baseURL = baseURL }
}

// GitHubRepo navigates to a GitHub repository page and extracts metadata.
func (b *Browser) GitHubRepo(owner, name string, opts ...GitHubOption) (*GitHubRepo, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, name)

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

		return repo;
	}`)
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval repo: %w", err)
	}

	repo := &GitHubRepo{
		Owner: owner,
		Name:  name,
	}

	if m, ok := result.Value.(map[string]any); ok {
		if v, ok := m["description"].(string); ok {
			repo.Description = v
		}

		if v, ok := m["stars"].(float64); ok {
			repo.Stars = int(v)
		}

		if v, ok := m["forks"].(float64); ok {
			repo.Forks = int(v)
		}

		if v, ok := m["language"].(string); ok {
			repo.Language = v
		}

		if v, ok := m["license"].(string); ok {
			repo.License = v
		}

		if topics, ok := m["topics"].([]any); ok {
			for _, t := range topics {
				if s, ok := t.(string); ok {
					repo.Topics = append(repo.Topics, s)
				}
			}
		}
	}

	// Optionally fetch README
	if cfg.includeBody {
		readmeURL := fmt.Sprintf("https://github.com/%s/%s/blob/HEAD/README.md", owner, name)

		readmePage, readmeErr := b.NewPage(readmeURL)
		if readmeErr == nil {
			defer func() { _ = readmePage.Close() }()

			if readmePage.WaitLoad() == nil {
				md, mdErr := readmePage.Markdown(WithMainContentOnly())
				if mdErr == nil {
					repo.ReadmeMD = md
				}
			}
		}
	}

	return repo, nil
}

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

// GitHubUser navigates to a GitHub user profile and extracts metadata.
func (b *Browser) GitHubUser(username string) (*GitHubUser, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	profileURL := fmt.Sprintf("https://github.com/%s", username)

	page, err := b.NewPage(profileURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate user: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	result, err := page.Eval(`() => {
		const user = {};

		// Display name
		const nameEl = document.querySelector('[itemprop="name"], .p-name');
		user.display_name = nameEl ? nameEl.textContent.trim() : '';

		// Username
		const loginEl = document.querySelector('[itemprop="additionalName"], .p-nickname');
		user.username = loginEl ? loginEl.textContent.trim() : '';

		// Bio
		const bioEl = document.querySelector('[data-bio-text], .p-note .user-profile-bio, [itemprop="description"]');
		user.bio = bioEl ? bioEl.textContent.trim() : '';

		// Location
		const locEl = document.querySelector('[itemprop="homeLocation"], .p-label');
		user.location = locEl ? locEl.textContent.trim() : '';

		// Counters from profile nav
		user.repos = 0;
		user.followers = 0;
		user.following = 0;

		// Try nav tabs
		document.querySelectorAll('a.UnderlineNav-item, nav a[data-tab-item]').forEach(a => {
			const text = a.textContent.trim().toLowerCase();
			const countEl = a.querySelector('.Counter, span.Counter');
			const count = countEl ? parseInt(countEl.textContent.trim().replace(/,/g, ''), 10) || 0 : 0;
			if (text.includes('repositories') || text.includes('repos')) {
				user.repos = count;
			}
		});

		// Followers/following from profile sidebar
		document.querySelectorAll('a[href*="tab=followers"], a[href*="tab=following"]').forEach(a => {
			const text = a.textContent.trim();
			const match = text.match(/([\d,.]+[km]?)/i);
			if (!match) return;
			let val = match[1].replace(/,/g, '');
			let num = 0;
			if (val.toLowerCase().endsWith('k')) {
				num = Math.round(parseFloat(val) * 1000);
			} else if (val.toLowerCase().endsWith('m')) {
				num = Math.round(parseFloat(val) * 1000000);
			} else {
				num = parseInt(val, 10) || 0;
			}
			if (a.href.includes('followers')) {
				user.followers = num;
			} else {
				user.following = num;
			}
		});

		return user;
	}`)
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval user: %w", err)
	}

	user := &GitHubUser{Username: username}

	if m, ok := result.Value.(map[string]any); ok {
		if v, ok := m["username"].(string); ok && v != "" {
			user.Username = v
		}

		if v, ok := m["display_name"].(string); ok {
			user.DisplayName = v
		}

		if v, ok := m["bio"].(string); ok {
			user.Bio = v
		}

		if v, ok := m["location"].(string); ok {
			user.Location = v
		}

		if v, ok := m["repos"].(float64); ok {
			user.Repos = int(v)
		}

		if v, ok := m["followers"].(float64); ok {
			user.Followers = int(v)
		}

		if v, ok := m["following"].(float64); ok {
			user.Following = int(v)
		}
	}

	return user, nil
}

// GitHubReleases navigates to the releases page of a GitHub repo and extracts release metadata.
func (b *Browser) GitHubReleases(owner, name string, opts ...GitHubOption) ([]GitHubRelease, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	releasesURL := fmt.Sprintf("https://github.com/%s/%s/releases", owner, name)

	page, err := b.NewPage(releasesURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate releases: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	result, err := page.Eval(fmt.Sprintf(`() => {
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

			// Date
			const timeEl = sec.querySelector('relative-time, time');
			rel.date = timeEl ? (timeEl.getAttribute('datetime') || timeEl.textContent.trim()) : '';

			// Assets count
			const assetEls = sec.querySelectorAll('.Box a[href*="/releases/download/"]');
			rel.assets = assetEls.length;

			// Body (short, from the description)
			const bodyEl = sec.querySelector('.markdown-body');
			rel.body = bodyEl ? bodyEl.textContent.trim().substring(0, 500) : '';

			items.push(rel);
		}
		return items;
	}`, cfg.maxItems))
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval releases: %w", err)
	}

	var releases []GitHubRelease

	if arr, ok := result.Value.([]any); ok {
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}

			rel := GitHubRelease{}
			if v, ok := m["tag"].(string); ok {
				rel.Tag = v
			}

			if v, ok := m["name"].(string); ok {
				rel.Name = v
			}

			if v, ok := m["body"].(string); ok {
				rel.Body = v
			}

			if v, ok := m["date"].(string); ok {
				rel.Date = v
			}

			if v, ok := m["assets"].(float64); ok {
				rel.Assets = int(v)
			}

			releases = append(releases, rel)
		}
	}

	return releases, nil
}

// GitHubTree navigates to a GitHub repo and extracts the file tree.
func (b *Browser) GitHubTree(owner, name, branch string) ([]string, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	if branch == "" {
		branch = "HEAD"
	}

	treeURL := fmt.Sprintf("https://github.com/%s/%s/find/%s", owner, name, branch)

	page, err := b.NewPage(treeURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate tree: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	result, err := page.Eval(`() => {
		const files = [];
		// The file finder page lists all files
		document.querySelectorAll('.tree-browser-result a, [data-testid="file-name-link"], .js-tree-browser-result-path').forEach(el => {
			const t = el.textContent.trim();
			if (t) files.push(t);
		});
		// Fallback: try the regular tree view
		if (files.length === 0) {
			document.querySelectorAll('.js-navigation-open[title], a.Link--primary[title]').forEach(el => {
				const t = el.getAttribute('title') || el.textContent.trim();
				if (t) files.push(t);
			});
		}
		return files;
	}`)
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval tree: %w", err)
	}

	var files []string

	if arr, ok := result.Value.([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					files = append(files, s)
				}
			}
		}
	}

	return files, nil
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
