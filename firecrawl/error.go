package firecrawl

import "fmt"

// APIError represents an error response from the Firecrawl API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("firecrawl: api error %d: %s", e.StatusCode, e.Message)
}

// AuthError indicates an authentication failure (401/403).
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("firecrawl: auth: %s", e.Message)
}

// RateLimitError indicates rate limiting (429).
type RateLimitError struct {
	RetryAfter int
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("firecrawl: rate limited: retry after %ds", e.RetryAfter)
	}

	return "firecrawl: rate limited"
}
