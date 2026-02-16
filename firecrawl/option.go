package firecrawl

import (
	"net/http"
	"time"
)

const defaultAPIURL = "https://api.firecrawl.dev/v1"

// Option configures the Firecrawl client.
type Option func(*Client)

// WithAPIURL sets a custom API base URL (e.g. self-hosted Firecrawl).
func WithAPIURL(url string) Option {
	return func(c *Client) { c.apiURL = url }
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}
