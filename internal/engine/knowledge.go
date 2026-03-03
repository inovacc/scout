package engine

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/inovacc/scout/pkg/scout/rod/lib/proto"
)

// KnowledgeResult holds the complete knowledge collection for a site.
type KnowledgeResult struct {
	URL       string           `json:"url"`
	Domain    string           `json:"domain"`
	CrawledAt time.Time        `json:"crawled_at"`
	Duration  string           `json:"duration"`
	TechStack *TechStack       `json:"tech_stack,omitempty"`
	Sitemap   []SitemapURL     `json:"sitemap,omitempty"`
	Pages     []KnowledgePage  `json:"pages"`
	Summary   KnowledgeSummary `json:"summary"`
}

// KnowledgePage holds all intelligence collected from a single page.
type KnowledgePage struct {
	URL        string          `json:"url"`
	Title      string          `json:"title"`
	Depth      int             `json:"depth"`
	Markdown   string          `json:"markdown"`
	HTML       string          `json:"html"`
	Links      []string        `json:"links"`
	Meta       *MetaData       `json:"meta,omitempty"`
	Cookies    []Cookie        `json:"cookies"`
	Screenshot string          `json:"screenshot"`
	Snapshot   string          `json:"snapshot"`
	HAR        []byte          `json:"har,omitempty"`
	HAREntries int             `json:"har_entries"`
	Frameworks []FrameworkInfo `json:"frameworks"`
	PageInfo   *PageInfo       `json:"page_info,omitempty"`
	ConsoleLog []string        `json:"console_log"`
	Swagger    *SwaggerSpec    `json:"swagger,omitempty"`
	PDF        []byte          `json:"pdf,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// KnowledgeSummary provides aggregate stats for the knowledge collection.
type KnowledgeSummary struct {
	PagesTotal   int           `json:"pages_total"`
	PagesSuccess int           `json:"pages_success"`
	PagesFailed  int           `json:"pages_failed"`
	UniqueLinks  int           `json:"unique_links"`
	Issues       []HealthIssue `json:"issues,omitempty"`
}

// Knowledge crawls a site and collects all possible intelligence per page.
func (b *Browser) Knowledge(targetURL string, opts ...KnowledgeOption) (*KnowledgeResult, error) {
	o := knowledgeDefaults()
	for _, fn := range opts {
		fn(o)
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("scout: knowledge: invalid URL: %w", err)
	}

	start := time.Now()
	result := &KnowledgeResult{
		URL:       targetURL,
		Domain:    parsed.Hostname(),
		CrawledAt: start,
	}

	// Try to fetch sitemap.
	sitemapURL := fmt.Sprintf("%s://%s/sitemap.xml", parsed.Scheme, parsed.Host)
	result.Sitemap, _ = b.ParseSitemap(sitemapURL)

	// Set up directory writer if output dir specified.
	var writer *KnowledgeWriter
	if o.outputDir != "" {
		writer = NewKnowledgeWriter(o.outputDir)
		if err := writer.Init(); err != nil {
			return nil, fmt.Errorf("scout: knowledge: init writer: %w", err)
		}
	}

	var (
		techStack *TechStack
		techOnce  sync.Once
		mu        sync.Mutex
		allLinks  = make(map[string]bool)
		issues    []HealthIssue
	)

	handler := func(page *Page, cr *CrawlResult) error {
		kp := KnowledgePage{
			URL:   cr.URL,
			Title: cr.Title,
			Depth: cr.Depth,
			Links: cr.Links,
		}

		// Detect tech stack on first page only.
		techOnce.Do(func() {
			techStack, _ = page.DetectTechStack()
		})

		_ = page.WaitFrameworkReady()

		// HAR recorder.
		var recorder *HijackRecorder
		hijacker, hijackErr := page.NewSessionHijacker(WithHijackBodyCapture())
		if hijackErr == nil {
			recorder = NewHijackRecorder()
			go recorder.RecordAll(hijacker.Events())
			defer hijacker.Stop()
		}

		// Console capture.
		var consoleLog []string
		rodPage := page.RodPage()
		rodPage.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
			msg := consoleArgsToString(e.Args)
			consoleLog = append(consoleLog, fmt.Sprintf("[%s] %s", e.Type, msg))
		})
		_ = proto.RuntimeEnable{}.Call(rodPage)

		kp.Meta, _ = page.ExtractMeta()
		kp.Cookies, _ = page.GetCookies()
		kp.Frameworks, _ = page.DetectFrameworks()
		kp.PageInfo, _ = page.CollectInfo()
		kp.HTML, _ = page.HTML()
		kp.Markdown, _ = page.Markdown()

		if data, scrErr := page.ScreenshotPNG(); scrErr == nil {
			kp.Screenshot = base64.StdEncoding.EncodeToString(data)
		}

		kp.Snapshot, _ = page.Snapshot()
		kp.PDF, _ = page.PDF()
		kp.Swagger, _ = page.ExtractSwagger()
		kp.ConsoleLog = consoleLog

		if recorder != nil {
			time.Sleep(500 * time.Millisecond)
			if harData, count, harErr := recorder.ExportHAR(); harErr == nil {
				kp.HAR = harData
				kp.HAREntries = count
			}
		}

		// Collect health issues from console errors.
		for _, msg := range consoleLog {
			if len(msg) > 7 && msg[:7] == "[error]" {
				issues = append(issues, HealthIssue{
					URL:      cr.URL,
					Source:   "console",
					Severity: "error",
					Message:  msg,
				})
			}
		}

		mu.Lock()
		for _, link := range cr.Links {
			allLinks[link] = true
		}
		result.Pages = append(result.Pages, kp)
		mu.Unlock()

		if writer != nil {
			_ = writer.WritePage(&kp)
		}

		return nil
	}

	_, err = b.Crawl(targetURL, handler,
		WithCrawlMaxDepth(o.maxDepth),
		WithCrawlMaxPages(o.maxPages),
		WithCrawlConcurrent(o.concurrency),
	)
	if err != nil {
		return nil, fmt.Errorf("scout: knowledge: crawl: %w", err)
	}

	result.TechStack = techStack
	result.Duration = time.Since(start).Round(time.Millisecond).String()

	var failed int
	for _, p := range result.Pages {
		if p.Error != "" {
			failed++
		}
	}

	result.Summary = KnowledgeSummary{
		PagesTotal:   len(result.Pages),
		PagesSuccess: len(result.Pages) - failed,
		PagesFailed:  failed,
		UniqueLinks:  len(allLinks),
		Issues:       issues,
	}

	if writer != nil {
		_ = writer.WriteManifest(result)
	}

	return result, nil
}
