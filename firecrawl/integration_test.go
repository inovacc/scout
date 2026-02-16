//go:build integration

package firecrawl

import (
	"context"
	"os"
	"testing"
)

func TestIntegrationScrape(t *testing.T) {
	key := os.Getenv("FIRECRAWL_API_KEY")
	if key == "" {
		t.Skip("FIRECRAWL_API_KEY not set")
	}

	client, err := New(key)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	doc, err := client.Scrape(context.Background(), "https://example.com",
		WithFormats(FormatMarkdown),
	)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}

	if doc.Markdown == "" {
		t.Error("expected markdown content")
	}
	t.Logf("title: %s", doc.Metadata.Title)
	t.Logf("markdown length: %d", len(doc.Markdown))
}
