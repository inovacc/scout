package firecrawl

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	return ts
}

func newTestClient(t *testing.T, ts *httptest.Server) *Client {
	t.Helper()

	c, err := New("test-key", WithAPIURL(ts.URL))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	return c
}
