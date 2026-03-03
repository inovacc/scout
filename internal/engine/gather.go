package engine

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout/rod/lib/proto"
)

// GatherResult holds all data collected from a single page in one pass.
type GatherResult struct {
	URL         string          `json:"url"`
	Title       string          `json:"title"`
	Meta        *MetaData       `json:"meta,omitempty"`
	Links       []string        `json:"links,omitempty"`
	Cookies     []Cookie        `json:"cookies,omitempty"`
	HTML        string          `json:"html,omitempty"`
	Markdown    string          `json:"markdown,omitempty"`
	Snapshot    string          `json:"snapshot,omitempty"`
	Screenshot  string          `json:"screenshot,omitempty"` // base64-encoded PNG
	HAR         []byte          `json:"har,omitempty"`
	HAREntries  int             `json:"har_entries,omitempty"`
	Frameworks  []FrameworkInfo `json:"frameworks,omitempty"`
	PageInfo    *PageInfo       `json:"page_info,omitempty"`
	ConsoleLog  []string        `json:"console_log,omitempty"`
	Duration    string          `json:"duration"`
	CollectedAt time.Time       `json:"collected_at"`
}

// GatherOption configures a Gather operation.
type GatherOption func(*gatherOptions)

type gatherOptions struct {
	html       bool
	markdown   bool
	screenshot bool
	snapshot   bool
	har        bool
	links      bool
	cookies    bool
	meta       bool
	frameworks bool
	pageInfo   bool
	console    bool
	all        bool
	timeout    time.Duration
}

func gatherDefaults() *gatherOptions {
	return &gatherOptions{
		all:     true,
		timeout: 30 * time.Second,
	}
}

// WithGatherHTML includes raw HTML in the result.
func WithGatherHTML() GatherOption {
	return func(o *gatherOptions) { o.html = true; o.all = false }
}

// WithGatherMarkdown includes markdown conversion in the result.
func WithGatherMarkdown() GatherOption {
	return func(o *gatherOptions) { o.markdown = true; o.all = false }
}

// WithGatherScreenshot includes a base64-encoded PNG screenshot.
func WithGatherScreenshot() GatherOption {
	return func(o *gatherOptions) { o.screenshot = true; o.all = false }
}

// WithGatherSnapshot includes accessibility tree snapshot.
func WithGatherSnapshot() GatherOption {
	return func(o *gatherOptions) { o.snapshot = true; o.all = false }
}

// WithGatherHAR enables HAR recording during page load.
func WithGatherHAR() GatherOption {
	return func(o *gatherOptions) { o.har = true; o.all = false }
}

// WithGatherLinks includes extracted links.
func WithGatherLinks() GatherOption {
	return func(o *gatherOptions) { o.links = true; o.all = false }
}

// WithGatherCookies includes page cookies.
func WithGatherCookies() GatherOption {
	return func(o *gatherOptions) { o.cookies = true; o.all = false }
}

// WithGatherMeta includes page metadata (OG, Twitter, JSON-LD).
func WithGatherMeta() GatherOption {
	return func(o *gatherOptions) { o.meta = true; o.all = false }
}

// WithGatherFrameworks includes detected frontend frameworks.
func WithGatherFrameworks() GatherOption {
	return func(o *gatherOptions) { o.frameworks = true; o.all = false }
}

// WithGatherConsole captures console output during page load.
func WithGatherConsole() GatherOption {
	return func(o *gatherOptions) { o.console = true; o.all = false }
}

// WithGatherTimeout sets the page load timeout. Default: 30s.
func WithGatherTimeout(d time.Duration) GatherOption {
	return func(o *gatherOptions) { o.timeout = d }
}

// Gather navigates to targetURL and collects all requested page intelligence
// in a single pass. By default, all data types are collected. Use specific
// WithGather* options to collect only what you need.
func (b *Browser) Gather(targetURL string, opts ...GatherOption) (*GatherResult, error) {
	o := gatherDefaults()
	for _, fn := range opts {
		fn(o)
	}

	start := time.Now()
	result := &GatherResult{
		URL:         targetURL,
		CollectedAt: start,
	}

	wantAll := o.all
	wantHAR := wantAll || o.har
	wantConsole := wantAll || o.console

	// Create page with session hijacker for HAR if needed.
	var pageOpts []Option
	if wantHAR {
		pageOpts = append(pageOpts, WithSessionHijack())
	}

	// We can't add options to b after creation, so use hijacker manually.
	page, err := b.NewPage("")
	if err != nil {
		return nil, fmt.Errorf("scout: gather: create page: %w", err)
	}

	defer func() { _ = page.Close() }()

	_ = pageOpts // hijack setup below

	// Set up HAR recorder.
	var recorder *HijackRecorder
	if wantHAR {
		hijacker, hijackErr := page.NewSessionHijacker(WithHijackBodyCapture())
		if hijackErr == nil {
			recorder = NewHijackRecorder()
			go recorder.RecordAll(hijacker.Events())

			defer hijacker.Stop()
		}
	}

	// Set up console capture.
	var consoleLog []string
	if wantConsole {
		rodPage := page.RodPage()
		rodPage.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
			msg := consoleArgsToString(e.Args)
			consoleLog = append(consoleLog, fmt.Sprintf("[%s] %s", e.Type, msg))
		})

		_ = proto.RuntimeEnable{}.Call(rodPage)
	}

	// Navigate.
	if err := page.Navigate(targetURL); err != nil {
		return nil, fmt.Errorf("scout: gather: navigate: %w", err)
	}

	_ = page.WaitLoad()
	_ = page.WaitFrameworkReady()

	// Collect basic info.
	result.Title, _ = page.Title()
	result.URL, _ = page.URL()

	// Page info (always lightweight).
	if wantAll || o.pageInfo {
		result.PageInfo, _ = page.CollectInfo()
	}

	// Meta.
	if wantAll || o.meta {
		result.Meta, _ = page.ExtractMeta()
	}

	// Links.
	if wantAll || o.links {
		result.Links, _ = page.ExtractLinks()
	}

	// Cookies.
	if wantAll || o.cookies {
		result.Cookies, _ = page.GetCookies()
	}

	// Frameworks.
	if wantAll || o.frameworks {
		result.Frameworks, _ = page.DetectFrameworks()
	}

	// HTML.
	if wantAll || o.html {
		result.HTML, _ = page.HTML()
	}

	// Markdown.
	if wantAll || o.markdown {
		result.Markdown, _ = page.Markdown()
	}

	// Screenshot.
	if wantAll || o.screenshot {
		if data, err := page.ScreenshotPNG(); err == nil {
			result.Screenshot = base64.StdEncoding.EncodeToString(data)
		}
	}

	// Snapshot (accessibility tree).
	if wantAll || o.snapshot {
		result.Snapshot, _ = page.Snapshot()
	}

	// HAR.
	if recorder != nil {
		// Small delay for late network events.
		time.Sleep(500 * time.Millisecond)

		if harData, count, err := recorder.ExportHAR(); err == nil {
			result.HAR = harData
			result.HAREntries = count
		}
	}

	// Console.
	if wantConsole {
		result.ConsoleLog = consoleLog
	}

	result.Duration = time.Since(start).Round(time.Millisecond).String()

	return result, nil
}
