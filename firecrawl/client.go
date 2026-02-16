package firecrawl

import (
	"fmt"
	"net/http"
	"time"
)

// Client is a Firecrawl API client.
type Client struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
}

// New creates a new Firecrawl client with the given API key.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("firecrawl: api key is required")
	}

	c := &Client{
		apiKey: apiKey,
		apiURL: defaultAPIURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}
