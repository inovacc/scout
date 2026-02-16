package firecrawl

// Format represents an output format for scraped content.
type Format string

const (
	FormatMarkdown         Format = "markdown"
	FormatHTML             Format = "html"
	FormatRawHTML          Format = "rawHtml"
	FormatLinks            Format = "links"
	FormatScreenshot       Format = "screenshot"
	FormatScreenshotFull   Format = "screenshot@fullPage"
)

// Document represents a scraped web page.
type Document struct {
	Markdown   string            `json:"markdown,omitempty"`
	HTML       string            `json:"html,omitempty"`
	RawHTML    string            `json:"rawHtml,omitempty"`
	Links      []string          `json:"links,omitempty"`
	Screenshot string            `json:"screenshot,omitempty"`
	Metadata   DocumentMetadata  `json:"metadata"`
}

// DocumentMetadata contains page metadata from a scrape.
type DocumentMetadata struct {
	Title         string `json:"title,omitempty"`
	Description   string `json:"description,omitempty"`
	Language      string `json:"language,omitempty"`
	URL           string `json:"url,omitempty"`
	SourceURL     string `json:"sourceURL,omitempty"`
	StatusCode    int    `json:"statusCode,omitempty"`
	OGTitle       string `json:"ogTitle,omitempty"`
	OGDescription string `json:"ogDescription,omitempty"`
	OGImage       string `json:"ogImage,omitempty"`
	OGLocale      string `json:"ogLocaleAlternate,omitempty"`
	OGURL         string `json:"ogUrl,omitempty"`
	OGSiteName    string `json:"ogSiteName,omitempty"`
}

// CrawlJob represents the status of an async crawl job.
type CrawlJob struct {
	Success    bool       `json:"success"`
	ID         string     `json:"id,omitempty"`
	Status     string     `json:"status,omitempty"`
	Total      int        `json:"total,omitempty"`
	Completed  int        `json:"completed,omitempty"`
	ExpiresAt  string     `json:"expiresAt,omitempty"`
	Data       []Document `json:"data,omitempty"`
	Next       string     `json:"next,omitempty"`
}

// BatchJob represents the status of an async batch scrape job.
type BatchJob struct {
	Success    bool       `json:"success"`
	ID         string     `json:"id,omitempty"`
	Status     string     `json:"status,omitempty"`
	Total      int        `json:"total,omitempty"`
	Completed  int        `json:"completed,omitempty"`
	ExpiresAt  string     `json:"expiresAt,omitempty"`
	Data       []Document `json:"data,omitempty"`
	Next       string     `json:"next,omitempty"`
}

// SearchResult represents the result of a search query.
type SearchResult struct {
	Success bool       `json:"success"`
	Data    []Document `json:"data,omitempty"`
}

// MapResult represents the result of a URL map operation.
type MapResult struct {
	Success bool     `json:"success"`
	Links   []string `json:"links,omitempty"`
}

// ExtractResult represents the result of an AI extraction.
type ExtractResult struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
}
