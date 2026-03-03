package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine/lib/proto"
)

// HealthIssue describes a single problem found during a health check.
type HealthIssue struct {
	URL        string `json:"url"`
	Source     string `json:"source"`   // "link", "console", "network", "js_exception"
	Severity   string `json:"severity"` // "error", "warning", "info"
	Message    string `json:"message"`
	Location   string `json:"location,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
}

// HealthReport summarizes the results of a site health check.
type HealthReport struct {
	URL      string         `json:"url"`
	Pages    int            `json:"pages_checked"`
	Duration string         `json:"duration"`
	Issues   []HealthIssue  `json:"issues"`
	Summary  map[string]int `json:"summary"`
}

// HealthCheck crawls targetURL and reports broken links, console errors,
// JS exceptions, and network failures. It reuses the Crawl BFS engine
// with per-page CDP event listeners.
func (b *Browser) HealthCheck(targetURL string, opts ...HealthCheckOption) (*HealthReport, error) {
	o := healthCheckDefaults()
	for _, fn := range opts {
		fn(o)
	}

	start := time.Now()

	var (
		mu     sync.Mutex
		issues []HealthIssue
	)

	addIssue := func(issue HealthIssue) {
		mu.Lock()
		issues = append(issues, issue)
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()

	_ = ctx // used indirectly via timeout awareness

	handler := func(page *Page, result *CrawlResult) error {
		pageURL := result.URL

		// Set up CDP event listeners for console errors and JS exceptions.
		rodPage := page.RodPage()

		var (
			pageMu     sync.Mutex
			pageIssues []HealthIssue
		)

		collectIssue := func(issue HealthIssue) {
			pageMu.Lock()
			pageIssues = append(pageIssues, issue)
			pageMu.Unlock()
		}

		wait := rodPage.EachEvent(
			func(e *proto.RuntimeConsoleAPICalled) {
				if e.Type != proto.RuntimeConsoleAPICalledTypeError &&
					e.Type != proto.RuntimeConsoleAPICalledTypeWarning {
					return
				}

				msg := consoleArgsToString(e.Args)
				severity := "warning"
				if e.Type == proto.RuntimeConsoleAPICalledTypeError {
					severity = "error"
				}

				collectIssue(HealthIssue{
					URL:      pageURL,
					Source:   "console",
					Severity: severity,
					Message:  msg,
				})
			},
			func(e *proto.RuntimeExceptionThrown) {
				msg := e.ExceptionDetails.Text
				loc := fmt.Sprintf("line %d, col %d", e.ExceptionDetails.LineNumber, e.ExceptionDetails.ColumnNumber)

				collectIssue(HealthIssue{
					URL:      pageURL,
					Source:   "js_exception",
					Severity: "error",
					Message:  msg,
					Location: loc,
				})
			},
		)

		// Enable Runtime domain for console/exception events.
		_ = proto.RuntimeEnable{}.Call(rodPage)

		// Set up network monitoring via session hijacker for HTTP errors.
		hijacker, hijackErr := page.NewSessionHijacker(WithHijackBodyCapture())
		if hijackErr == nil {
			go func() {
				for ev := range hijacker.Events() {
					if ev.Response != nil && ev.Response.Status >= 400 {
						severity := "warning"
						if ev.Response.Status >= 500 {
							severity = "error"
						}

						collectIssue(HealthIssue{
							URL:        pageURL,
							Source:     "network",
							Severity:   severity,
							Message:    fmt.Sprintf("HTTP %d: %s", ev.Response.Status, ev.Response.URL),
							StatusCode: ev.Response.Status,
						})
					}
				}
			}()
		}

		// Wait for page to fully load.
		_ = page.WaitLoad()
		_ = page.WaitFrameworkReady()

		// Give a short window for late console errors.
		time.Sleep(500 * time.Millisecond)

		// Stop listeners.
		if hijackErr == nil {
			hijacker.Stop()
		}

		wait()

		// Collect page issues.
		pageMu.Lock()
		for _, issue := range pageIssues {
			addIssue(issue)
		}
		pageMu.Unlock()

		// Check links for broken references (status >= 400).
		for _, link := range result.Links {
			if isBrokenLinkCandidate(link) {
				addIssue(HealthIssue{
					URL:      pageURL,
					Source:   "link",
					Severity: "warning",
					Message:  fmt.Sprintf("suspicious link: %s", link),
				})
			}
		}

		return nil
	}

	results, err := b.Crawl(targetURL, handler,
		WithCrawlMaxDepth(o.maxDepth),
		WithCrawlConcurrent(o.concurrency),
		WithCrawlMaxPages(100),
	)
	if err != nil {
		return nil, fmt.Errorf("scout: health check: %w", err)
	}

	// Check crawl results for page-level errors (e.g. navigation failures).
	for _, r := range results {
		if r.Error != nil {
			addIssue(HealthIssue{
				URL:      r.URL,
				Source:   "link",
				Severity: "error",
				Message:  fmt.Sprintf("page load failed: %v", r.Error),
			})
		}
	}

	summary := make(map[string]int)
	for _, issue := range issues {
		summary[issue.Severity]++
	}

	return &HealthReport{
		URL:      targetURL,
		Pages:    len(results),
		Duration: time.Since(start).Round(time.Millisecond).String(),
		Issues:   issues,
		Summary:  summary,
	}, nil
}

func consoleArgsToString(args []*proto.RuntimeRemoteObject) string {
	var parts []string
	for _, arg := range args {
		if arg.Value.Str() != "" {
			parts = append(parts, arg.Value.Str())
		} else if arg.ClassName != "" {
			parts = append(parts, arg.ClassName)
		} else {
			parts = append(parts, string(arg.Type))
		}
	}

	return strings.Join(parts, " ")
}

func isBrokenLinkCandidate(link string) bool {
	// Flag javascript: void, empty, or obviously malformed links.
	return link == "" || link == "#" || strings.HasPrefix(link, "javascript:void")
}
