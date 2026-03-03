package engine

import (
	"testing"
	"time"
)

func TestResearchCache_PutGet(t *testing.T) {
	cache := NewResearchCache(1 * time.Hour)

	result := &ResearchResult{
		Query:   "test query",
		Summary: "test summary",
		Sources: []ResearchSource{{URL: "https://example.com", Title: "Example"}},
	}

	cache.Put("test query", result)

	got, ok := cache.Get("test query")
	if !ok {
		t.Fatal("expected cache hit")
	}

	if got.Summary != "test summary" {
		t.Fatalf("expected 'test summary', got %q", got.Summary)
	}
}

func TestResearchCache_Expiry(t *testing.T) {
	cache := NewResearchCache(50 * time.Millisecond)

	cache.Put("query", &ResearchResult{Summary: "old"})

	if _, ok := cache.Get("query"); !ok {
		t.Fatal("expected cache hit before expiry")
	}

	time.Sleep(60 * time.Millisecond)

	if _, ok := cache.Get("query"); ok {
		t.Fatal("expected cache miss after expiry")
	}
}

func TestResearchCache_Evict(t *testing.T) {
	cache := NewResearchCache(50 * time.Millisecond)

	cache.Put("a", &ResearchResult{Summary: "a"})
	cache.Put("b", &ResearchResult{Summary: "b"})

	if cache.Size() != 2 {
		t.Fatalf("expected size 2, got %d", cache.Size())
	}

	time.Sleep(60 * time.Millisecond)

	evicted := cache.Evict()
	if evicted != 2 {
		t.Fatalf("expected 2 evicted, got %d", evicted)
	}

	if cache.Size() != 0 {
		t.Fatalf("expected size 0 after evict, got %d", cache.Size())
	}
}

func TestResearchCache_Clear(t *testing.T) {
	cache := NewResearchCache(1 * time.Hour)
	cache.Put("a", &ResearchResult{Summary: "a"})
	cache.Put("b", &ResearchResult{Summary: "b"})
	cache.Clear()

	if cache.Size() != 0 {
		t.Fatalf("expected size 0, got %d", cache.Size())
	}
}

func TestResearchCache_Miss(t *testing.T) {
	cache := NewResearchCache(1 * time.Hour)
	if _, ok := cache.Get("nonexistent"); ok {
		t.Fatal("expected cache miss")
	}
}

func TestResearchPreset_Shallow(t *testing.T) {
	o := researchDefaults()
	WithResearchPreset(ResearchShallow)(o)

	if o.maxDepth != 1 || o.maxSources != 3 || o.concurrency != 2 {
		t.Fatalf("shallow preset: depth=%d sources=%d concurrency=%d", o.maxDepth, o.maxSources, o.concurrency)
	}
}

func TestResearchPreset_Medium(t *testing.T) {
	o := researchDefaults()
	WithResearchPreset(ResearchMedium)(o)

	if o.maxDepth != 2 || o.maxSources != 5 || o.concurrency != 3 {
		t.Fatalf("medium preset: depth=%d sources=%d concurrency=%d", o.maxDepth, o.maxSources, o.concurrency)
	}
}

func TestResearchPreset_Deep(t *testing.T) {
	o := researchDefaults()
	WithResearchPreset(ResearchDeep)(o)

	if o.maxDepth != 3 || o.maxSources != 8 || o.concurrency != 5 {
		t.Fatalf("deep preset: depth=%d sources=%d concurrency=%d", o.maxDepth, o.maxSources, o.concurrency)
	}

	if o.timeout != 5*time.Minute {
		t.Fatalf("deep preset timeout: got %v", o.timeout)
	}
}
