package engine

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
