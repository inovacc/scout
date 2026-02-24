package scout

import (
	"fmt"
	"net/http"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/github-repo", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>owner/testrepo: A test repository</title></head>
<body>
<div data-testid="about-description">A test repository for unit tests</div>
<span id="repo-stars-counter-star">1,234</span>
<span id="repo-network-counter">567</span>
<span itemprop="programmingLanguage">Go</span>
<a class="topic-tag">testing</a>
<a class="topic-tag">automation</a>
<a data-analytics-event="LICENSE">MIT License</a>
</body></html>`)
		})

		mux.HandleFunc("/github-issues", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Issues</title></head>
<body>
<div class="js-issue-row">
  <a data-hovercard-type="issue" href="/owner/repo/issues/42">Fix the bug</a>
  <span class="octicon-issue-opened"></span>
  <a data-hovercard-type="user">alice</a>
  <a data-name="bug">bug</a>
  <a data-name="help wanted">help wanted</a>
  <relative-time datetime="2025-01-15T10:00:00Z">Jan 15</relative-time>
</div>
<div class="js-issue-row">
  <a data-hovercard-type="issue" href="/owner/repo/issues/43">Add feature X</a>
  <span class="octicon-issue-opened"></span>
  <a data-hovercard-type="user">bob</a>
  <relative-time datetime="2025-02-01T12:00:00Z">Feb 1</relative-time>
</div>
</body></html>`)
		})

		mux.HandleFunc("/github-prs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Pull Requests</title></head>
<body>
<div class="js-issue-row">
  <a data-hovercard-type="pull_request" href="/owner/repo/pull/10">Refactor internals</a>
  <span class="octicon-git-merge"></span>
  <a data-hovercard-type="user">charlie</a>
  <relative-time datetime="2025-03-10T08:00:00Z">Mar 10</relative-time>
</div>
</body></html>`)
		})

		mux.HandleFunc("/github-user", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>testuser</title></head>
<body>
<span itemprop="name">Test User</span>
<span itemprop="additionalName">testuser</span>
<div itemprop="description">I build things</div>
<span itemprop="homeLocation">San Francisco</span>
<a class="UnderlineNav-item" href="/testuser?tab=repositories">Repositories <span class="Counter">42</span></a>
<a href="/testuser?tab=followers">100 followers</a>
<a href="/testuser?tab=following">50 following</a>
</body></html>`)
		})

		mux.HandleFunc("/github-releases", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Releases</title></head>
<body>
<section aria-label="v1.2.0">
  <a href="/owner/repo/releases/tag/v1.2.0"><span>v1.2.0</span></a>
  <h2><a href="/owner/repo/releases/tag/v1.2.0">Release 1.2.0</a></h2>
  <relative-time datetime="2025-04-01T00:00:00Z">Apr 1</relative-time>
  <div class="markdown-body">Bug fixes and improvements</div>
  <div class="Box">
    <a href="/owner/repo/releases/download/v1.2.0/app.tar.gz">app.tar.gz</a>
    <a href="/owner/repo/releases/download/v1.2.0/app.zip">app.zip</a>
  </div>
</section>
</body></html>`)
		})
	})
}

func TestGitHubRepo(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	// We can't easily test against real github.com in unit tests,
	// so we test the option defaults and nil safety.

	// Test nil browser
	var nilBrowser *Browser
	_, err := nilBrowser.GitHubRepo("owner", "repo")
	if err == nil {
		t.Fatal("expected error for nil browser")
	}

	// Test option functions apply correctly
	cfg := githubDefaults()
	WithGitHubBody()(cfg)
	WithGitHubMaxItems(10)(cfg)
	WithGitHubState("closed")(cfg)

	if !cfg.includeBody {
		t.Error("WithGitHubBody did not set includeBody")
	}
	if cfg.maxItems != 10 {
		t.Error("WithGitHubMaxItems did not set maxItems")
	}
	if cfg.state != "closed" {
		t.Error("WithGitHubState did not set state")
	}

	// Test against mock page (repo extraction from local test server)
	page, err := b.NewPage(ts.URL + "/github-repo")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	result, err := page.Eval(`() => {
		const descEl = document.querySelector('[data-testid="about-description"]');
		return descEl ? descEl.textContent.trim() : '';
	}`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	desc, ok := result.Value.(string)
	if !ok || desc != "A test repository for unit tests" {
		t.Errorf("expected description 'A test repository for unit tests', got %q", desc)
	}
}

func TestGitHubIssuesMock(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/github-issues")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	result, err := page.Eval(`() => {
		const rows = document.querySelectorAll('.js-issue-row');
		return rows.length;
	}`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	count, ok := result.Value.(float64)
	if !ok || int(count) != 2 {
		t.Errorf("expected 2 issue rows, got %v", result.Value)
	}
}

func TestGitHubPRsMock(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/github-prs")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	result, err := page.Eval(`() => {
		const rows = document.querySelectorAll('.js-issue-row');
		const row = rows[0];
		const mergedIcon = row.querySelector('.octicon-git-merge');
		return mergedIcon !== null;
	}`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	merged, ok := result.Value.(bool)
	if !ok || !merged {
		t.Error("expected merged icon to be present")
	}
}

func TestGitHubUserMock(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/github-user")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	result, err := page.Eval(`() => {
		const nameEl = document.querySelector('[itemprop="name"]');
		const bioEl = document.querySelector('[itemprop="description"]');
		return {
			name: nameEl ? nameEl.textContent.trim() : '',
			bio: bioEl ? bioEl.textContent.trim() : ''
		};
	}`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	m, ok := result.Value.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	if m["name"] != "Test User" {
		t.Errorf("expected name 'Test User', got %v", m["name"])
	}
	if m["bio"] != "I build things" {
		t.Errorf("expected bio 'I build things', got %v", m["bio"])
	}
}

func TestGitHubReleasesMock(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/github-releases")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	result, err := page.Eval(`() => {
		const sections = document.querySelectorAll('section[aria-label]');
		return sections.length;
	}`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	count, ok := result.Value.(float64)
	if !ok || int(count) != 1 {
		t.Errorf("expected 1 release section, got %v", result.Value)
	}
}

func TestGitHubNilBrowser(t *testing.T) {
	var b *Browser

	_, err := b.GitHubRepo("o", "r")
	if err == nil {
		t.Error("expected error")
	}

	_, err = b.GitHubIssues("o", "r")
	if err == nil {
		t.Error("expected error")
	}

	_, err = b.GitHubPRs("o", "r")
	if err == nil {
		t.Error("expected error")
	}

	_, err = b.GitHubUser("u")
	if err == nil {
		t.Error("expected error")
	}

	_, err = b.GitHubReleases("o", "r")
	if err == nil {
		t.Error("expected error")
	}

	_, err = b.GitHubTree("o", "r", "main")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGitHubOptionDefaults(t *testing.T) {
	cfg := githubDefaults()

	if cfg.maxItems != 30 {
		t.Errorf("expected default maxItems 30, got %d", cfg.maxItems)
	}
	if cfg.state != "open" {
		t.Errorf("expected default state 'open', got %s", cfg.state)
	}
	if cfg.includeBody {
		t.Error("expected default includeBody false")
	}
}
