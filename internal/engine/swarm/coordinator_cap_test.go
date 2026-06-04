package swarm

import (
	"fmt"
	"testing"
)

// TestEnqueueMaxURLsCap proves the coordinator stops admitting new URLs once
// the seen-set reaches MaxURLs, bounding memory against a sprawling/hostile
// crawl frontier.
func TestEnqueueMaxURLsCap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxURLs = 5

	c := NewCoordinator(cfg, nil)

	reqs := make([]CrawlRequest, 0, 20)
	for i := 0; i < 20; i++ {
		reqs = append(reqs, CrawlRequest{URL: fmt.Sprintf("https://example.com/%d", i)})
	}

	n, err := c.Enqueue(reqs)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if n != 5 {
		t.Errorf("Enqueue admitted %d URLs, want it capped at 5", n)
	}

	if len(c.seen) != 5 {
		t.Errorf("seen-set size = %d, want 5 (capped)", len(c.seen))
	}
}

// TestNewCoordinatorClampsMaxURLs proves a SwarmConfig that leaves MaxURLs unset
// is clamped to the fail-closed default, so the cap applies even on the gRPC
// daemon path where callers build the config directly.
func TestNewCoordinatorClampsMaxURLs(t *testing.T) {
	c := NewCoordinator(SwarmConfig{}, nil)
	if c.config.MaxURLs != defaultMaxURLs {
		t.Errorf("MaxURLs = %d, want clamp to default %d", c.config.MaxURLs, defaultMaxURLs)
	}
}
