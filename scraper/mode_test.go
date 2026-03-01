package scraper

import (
	"context"
	"sort"
	"testing"
	"time"
)

// mockSession implements SessionData for testing.
type mockSession struct {
	provider string
}

func (s *mockSession) ProviderName() string { return s.provider }

// mockAuthProvider implements AuthProvider for testing.
type mockAuthProvider struct {
	name string
}

func (p *mockAuthProvider) Name() string { return p.name }

// mockMode implements Mode for testing the registry and scrape pipeline.
type mockMode struct {
	name        string
	description string
	provider    *mockAuthProvider
	scrapeFunc  func(ctx context.Context, session SessionData, opts ScrapeOptions) (<-chan Result, error)
}

func (m *mockMode) Name() string               { return m.name }
func (m *mockMode) Description() string         { return m.description }
func (m *mockMode) AuthProvider() AuthProvider   { return m.provider }

func (m *mockMode) Scrape(ctx context.Context, session SessionData, opts ScrapeOptions) (<-chan Result, error) {
	if m.scrapeFunc != nil {
		return m.scrapeFunc(ctx, session, opts)
	}

	ch := make(chan Result)
	close(ch)
	return ch, nil
}

func TestModeRegistry(t *testing.T) {
	// Save and restore global registry.
	original := DefaultModeRegistry
	DefaultModeRegistry = &ModeRegistry{modes: make(map[string]Mode)}
	defer func() { DefaultModeRegistry = original }()

	m := &mockMode{name: "test", description: "Test mode", provider: &mockAuthProvider{name: "test"}}
	RegisterMode(m)

	got, err := GetMode("test")
	if err != nil {
		t.Fatalf("GetMode: %v", err)
	}
	if got.Name() != "test" {
		t.Errorf("Name = %q, want %q", got.Name(), "test")
	}
	if got.Description() != "Test mode" {
		t.Errorf("Description = %q, want %q", got.Description(), "Test mode")
	}
}

func TestModeRegistry_Unknown(t *testing.T) {
	original := DefaultModeRegistry
	DefaultModeRegistry = &ModeRegistry{modes: make(map[string]Mode)}
	defer func() { DefaultModeRegistry = original }()

	_, err := GetMode("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestListModes(t *testing.T) {
	original := DefaultModeRegistry
	DefaultModeRegistry = &ModeRegistry{modes: make(map[string]Mode)}
	defer func() { DefaultModeRegistry = original }()

	RegisterMode(&mockMode{name: "bravo", provider: &mockAuthProvider{name: "b"}})
	RegisterMode(&mockMode{name: "alpha", provider: &mockAuthProvider{name: "a"}})

	names := ListModes()
	sort.Strings(names)
	if len(names) != 2 || names[0] != "alpha" || names[1] != "bravo" {
		t.Fatalf("ListModes = %v, want [alpha bravo]", names)
	}
}

func TestModeRegistry_Overwrite(t *testing.T) {
	original := DefaultModeRegistry
	DefaultModeRegistry = &ModeRegistry{modes: make(map[string]Mode)}
	defer func() { DefaultModeRegistry = original }()

	RegisterMode(&mockMode{name: "dup", description: "first", provider: &mockAuthProvider{name: "dup"}})
	RegisterMode(&mockMode{name: "dup", description: "second", provider: &mockAuthProvider{name: "dup"}})

	got, _ := GetMode("dup")
	if got.Description() != "second" {
		t.Errorf("expected second registration to win, got %q", got.Description())
	}
}

func TestModeScrape_EmitsResults(t *testing.T) {
	m := &mockMode{
		name:     "emitter",
		provider: &mockAuthProvider{name: "emitter"},
		scrapeFunc: func(ctx context.Context, _ SessionData, opts ScrapeOptions) (<-chan Result, error) {
			ch := make(chan Result, 3)
			go func() {
				defer close(ch)
				for i := 0; i < 3; i++ {
					select {
					case ch <- Result{
						Type:    ResultPost,
						Source:  "test",
						ID:      string(rune('a' + i)),
						Content: "post content",
					}:
					case <-ctx.Done():
						return
					}
				}
			}()
			return ch, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := m.Scrape(ctx, &mockSession{provider: "emitter"}, DefaultScrapeOptions())
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}

	var results []Result
	for r := range ch {
		results = append(results, r)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if results[0].Type != ResultPost {
		t.Errorf("type = %q, want %q", results[0].Type, ResultPost)
	}
}

func TestModeScrape_ContextCancel(t *testing.T) {
	m := &mockMode{
		name:     "slow",
		provider: &mockAuthProvider{name: "slow"},
		scrapeFunc: func(ctx context.Context, _ SessionData, _ ScrapeOptions) (<-chan Result, error) {
			ch := make(chan Result)
			go func() {
				defer close(ch)
				// Emit results slowly.
				for i := 0; i < 100; i++ {
					select {
					case ch <- Result{Type: ResultMessage, ID: "m"}:
						time.Sleep(50 * time.Millisecond)
					case <-ctx.Done():
						return
					}
				}
			}()
			return ch, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	ch, err := m.Scrape(ctx, nil, DefaultScrapeOptions())
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}

	var count int
	for range ch {
		count++
	}
	// Should have gotten some but not all 100 results due to cancellation.
	if count >= 100 {
		t.Errorf("expected cancellation to limit results, got %d", count)
	}
}

func TestModeScrape_WithProgress(t *testing.T) {
	var updates []Progress
	opts := DefaultScrapeOptions()
	opts.Progress = func(p Progress) {
		updates = append(updates, p)
	}

	m := &mockMode{
		name:     "progress",
		provider: &mockAuthProvider{name: "progress"},
		scrapeFunc: func(_ context.Context, _ SessionData, opts ScrapeOptions) (<-chan Result, error) {
			if opts.Progress != nil {
				opts.Progress(Progress{Phase: "init", Message: "starting"})
				opts.Progress(Progress{Phase: "scraping", Current: 5, Total: 10, Message: "half done"})
				opts.Progress(Progress{Phase: "done", Current: 10, Total: 10, Message: "complete"})
			}
			ch := make(chan Result)
			close(ch)
			return ch, nil
		},
	}

	ch, err := m.Scrape(context.Background(), nil, opts)
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	for range ch {
	}

	if len(updates) != 3 {
		t.Fatalf("got %d progress updates, want 3", len(updates))
	}
	if updates[0].Phase != "init" || updates[2].Phase != "done" {
		t.Errorf("unexpected phases: %v", updates)
	}
}

func TestDefaultScrapeOptions(t *testing.T) {
	opts := DefaultScrapeOptions()
	if !opts.Headless {
		t.Error("expected Headless=true by default")
	}
	if !opts.Stealth {
		t.Error("expected Stealth=true by default")
	}
	if opts.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m", opts.Timeout)
	}
	if opts.Limit != 0 {
		t.Error("expected Limit=0 by default")
	}
}

func TestResultTypes(t *testing.T) {
	types := []ResultType{
		ResultMessage, ResultChannel, ResultThread, ResultUser,
		ResultFile, ResultReaction, ResultPost, ResultComment,
		ResultSubreddit, ResultMeeting, ResultEmail, ResultProfile,
		ResultMember, ResultPin,
	}

	seen := make(map[ResultType]bool)
	for _, rt := range types {
		if seen[rt] {
			t.Errorf("duplicate result type: %s", rt)
		}
		seen[rt] = true
		if string(rt) == "" {
			t.Error("empty result type constant")
		}
	}
}

func TestResult_Metadata(t *testing.T) {
	r := Result{
		Type:   ResultPost,
		Source: "test",
		ID:     "123",
		Metadata: map[string]any{
			"score":    42,
			"comments": 7,
		},
	}

	if r.Metadata["score"] != 42 {
		t.Errorf("score = %v, want 42", r.Metadata["score"])
	}
}
