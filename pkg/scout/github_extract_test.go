package scout

import (
	"fmt"
	"net/http"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/extract-github-repo", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>myowner/myrepo</title></head>
<body>
<div data-testid="about-description">A fantastic project</div>
<span id="repo-stars-counter-star">2,345</span>
<span id="repo-network-counter">789</span>
<span itemprop="programmingLanguage">Go</span>
<a class="topic-tag">scraping</a>
<a class="topic-tag">browser</a>
<a data-analytics-event="LICENSE">Apache-2.0</a>
<relative-time datetime="2025-06-01T12:00:00Z">Jun 1</relative-time>
<article class="markdown-body"><h1>My Repo</h1><p>Welcome to the project.</p></article>
</body></html>`)
		})

		mux.HandleFunc("/extract-github-issues", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Issues</title></head>
<body>
<div class="js-issue-row">
  <a data-hovercard-type="issue" href="/myowner/myrepo/issues/10">First issue</a>
  <a data-hovercard-type="user">bob</a>
  <a data-name="bug">bug</a>
  <relative-time datetime="2025-03-01T08:00:00Z">Mar 1</relative-time>
  <a aria-label="3 comments">3</a>
</div>
<div class="js-issue-row">
  <a data-hovercard-type="issue" href="/myowner/myrepo/issues/11">Second issue</a>
  <span class="octicon-issue-closed"></span>
  <a data-hovercard-type="user">carol</a>
  <relative-time datetime="2025-03-02T09:00:00Z">Mar 2</relative-time>
</div>
</body></html>`)
		})

		mux.HandleFunc("/extract-github-prs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Pull Requests</title></head>
<body>
<div class="js-issue-row">
  <a data-hovercard-type="pull_request" href="/myowner/myrepo/pull/50">Add feature Y</a>
  <a data-hovercard-type="user">dave</a>
  <a data-name="enhancement">enhancement</a>
  <relative-time datetime="2025-04-10T10:00:00Z">Apr 10</relative-time>
  <a aria-label="5 comments">5</a>
</div>
<div class="js-issue-row">
  <a data-hovercard-type="pull_request" href="/myowner/myrepo/pull/51">Fix typo</a>
  <span class="octicon-git-merge"></span>
  <a data-hovercard-type="user">eve</a>
  <relative-time datetime="2025-04-11T11:00:00Z">Apr 11</relative-time>
</div>
</body></html>`)
		})

		mux.HandleFunc("/extract-github-releases", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Releases</title></head>
<body>
<section aria-label="v1.2.0">
  <a href="/myowner/myrepo/releases/tag/v1.2.0"><span>v1.2.0</span></a>
  <h2><a>Release 1.2.0</a></h2>
  <relative-time datetime="2025-05-20T14:00:00Z">May 20</relative-time>
  <a data-hovercard-type="user">frank</a>
  <a href="/myowner/myrepo/releases/download/v1.2.0/app-linux">app-linux</a>
  <a href="/myowner/myrepo/releases/download/v1.2.0/app-darwin">app-darwin</a>
  <div class="markdown-body">Bug fixes and improvements.</div>
</section>
<section aria-label="v1.1.0">
  <a href="/myowner/myrepo/releases/tag/v1.1.0"><span>v1.1.0</span></a>
  <h2><a>Release 1.1.0</a></h2>
  <relative-time datetime="2025-04-15T10:00:00Z">Apr 15</relative-time>
  <a data-hovercard-type="user">frank</a>
  <div class="markdown-body">Initial stable release.</div>
</section>
</body></html>`)
		})
	})
}

func TestGitHubExtractRepoInfo(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	tests := []struct {
		name          string
		owner         string
		repo          string
		opts          []GitHubExtractOption
		wantDesc      string
		wantStars     int
		wantForks     int
		wantLang      string
		wantTopics    int
		wantLicense   string
		wantUpdated   bool
		wantReadmeLen int
	}{
		{
			name:        "basic repo extraction",
			owner:       "myowner",
			repo:        "myrepo",
			wantDesc:    "A fantastic project",
			wantStars:   2345,
			wantForks:   789,
			wantLang:    "Go",
			wantTopics:  2,
			wantLicense: "Apache-2.0",
			wantUpdated: true,
		},
		{
			name:          "with readme",
			owner:         "myowner",
			repo:          "myrepo",
			opts:          []GitHubExtractOption{WithGitHubReadme()},
			wantDesc:      "A fantastic project",
			wantStars:     2345,
			wantForks:     789,
			wantLang:      "Go",
			wantTopics:    2,
			wantLicense:   "Apache-2.0",
			wantUpdated:   true,
			wantReadmeLen: 1, // at least some content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We navigate to the mock page by using baseURL that points to the test route
			// Since the method constructs URL as baseURL/owner/repo, we use a trick:
			// set baseURL to ts.URL and serve on the constructed path
			opts := []GitHubExtractOption{withGitHubExtractBaseURL(ts.URL + "/extract-github-repo#")}
			opts = append(opts, tt.opts...)

			repo, err := b.GitHubExtractRepoInfo("myowner", "myrepo", opts...)
			if err != nil {
				t.Fatalf("GitHubExtractRepoInfo() error: %v", err)
			}

			if repo.Owner != "myowner" {
				t.Errorf("Owner = %q, want %q", repo.Owner, "myowner")
			}

			if repo.Name != "myrepo" {
				t.Errorf("Name = %q, want %q", repo.Name, "myrepo")
			}
		})
	}
}

func TestGitHubExtractIssues(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	issues, err := b.GitHubExtractIssues("myowner", "myrepo",
		withGitHubExtractBaseURL(ts.URL+"/extract-github-issues#"),
	)
	if err != nil {
		t.Fatalf("GitHubExtractIssues() error: %v", err)
	}

	if len(issues) < 1 {
		t.Fatalf("expected at least 1 issue, got %d", len(issues))
	}

	// Verify none are PRs
	for _, issue := range issues {
		if issue.IsPR {
			t.Errorf("issue #%d should not be a PR", issue.Number)
		}
	}
}

func TestGitHubExtractPRs(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	prs, err := b.GitHubExtractPRs("myowner", "myrepo",
		withGitHubExtractBaseURL(ts.URL+"/extract-github-prs#"),
	)
	if err != nil {
		t.Fatalf("GitHubExtractPRs() error: %v", err)
	}

	if len(prs) < 1 {
		t.Fatalf("expected at least 1 PR, got %d", len(prs))
	}

	// All should be PRs
	for _, pr := range prs {
		if !pr.IsPR {
			t.Errorf("PR #%d should have IsPR=true", pr.Number)
		}
	}
}

func TestGitHubExtractReleases(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	releases, err := b.GitHubExtractReleases("myowner", "myrepo",
		withGitHubExtractBaseURL(ts.URL+"/extract-github-releases#"),
	)
	if err != nil {
		t.Fatalf("GitHubExtractReleases() error: %v", err)
	}

	if len(releases) < 1 {
		t.Fatalf("expected at least 1 release, got %d", len(releases))
	}
}

func TestGitHubExtractMaxItems(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	issues, err := b.GitHubExtractIssues("myowner", "myrepo",
		withGitHubExtractBaseURL(ts.URL+"/extract-github-issues#"),
		WithGitHubExtractMaxItems(1),
	)
	if err != nil {
		t.Fatalf("GitHubExtractIssues() error: %v", err)
	}

	if len(issues) > 1 {
		t.Errorf("expected at most 1 issue with maxItems=1, got %d", len(issues))
	}
}

func TestGitHubExtractOptions(t *testing.T) {
	tests := []struct {
		name    string
		opt     GitHubExtractOption
		checkFn func(*githubExtractOpts) bool
	}{
		{
			name: "WithGitHubExtractBody",
			opt:  WithGitHubExtractBody(),
			checkFn: func(o *githubExtractOpts) bool {
				return o.includeBody
			},
		},
		{
			name: "WithGitHubReadme",
			opt:  WithGitHubReadme(),
			checkFn: func(o *githubExtractOpts) bool {
				return o.includeReadme
			},
		},
		{
			name: "WithGitHubExtractMaxItems",
			opt:  WithGitHubExtractMaxItems(10),
			checkFn: func(o *githubExtractOpts) bool {
				return o.maxItems == 10
			},
		},
		{
			name: "WithGitHubExtractState",
			opt:  WithGitHubExtractState("closed"),
			checkFn: func(o *githubExtractOpts) bool {
				return o.state == "closed"
			},
		},
		{
			name: "WithGitHubExtractMaxItems zero ignored",
			opt:  WithGitHubExtractMaxItems(0),
			checkFn: func(o *githubExtractOpts) bool {
				return o.maxItems == 25 // default preserved
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := githubExtractDefaults()
			tt.opt(opts)

			if !tt.checkFn(opts) {
				t.Errorf("option %s did not set expected value", tt.name)
			}
		})
	}
}

func TestGitHubExtractNilBrowser(t *testing.T) {
	var b *Browser

	_, err := b.GitHubExtractRepoInfo("owner", "repo")
	if err == nil {
		t.Fatal("expected error for nil browser")
	}

	_, err = b.GitHubExtractIssues("owner", "repo")
	if err == nil {
		t.Fatal("expected error for nil browser")
	}

	_, err = b.GitHubExtractPRs("owner", "repo")
	if err == nil {
		t.Fatal("expected error for nil browser")
	}

	_, err = b.GitHubExtractReleases("owner", "repo")
	if err == nil {
		t.Fatal("expected error for nil browser")
	}
}
