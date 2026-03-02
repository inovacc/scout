package runbook

import (
	"testing"
)

func TestScoreSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		wantTier string
		minScore float64
		maxScore float64
	}{
		// Excellent tier
		{
			name:     "data-testid attribute",
			selector: `[data-testid="product"]`,
			wantTier: "excellent",
			minScore: 0.9,
			maxScore: 1.0,
		},
		{
			name:     "data-id attribute",
			selector: `[data-id="123"]`,
			wantTier: "excellent",
			minScore: 0.9,
			maxScore: 1.0,
		},
		{
			name:     "ID selector",
			selector: "#main-content",
			wantTier: "excellent",
			minScore: 0.9,
			maxScore: 1.0,
		},
		{
			name:     "ID with descendant",
			selector: "#app .content",
			wantTier: "excellent",
			minScore: 0.9,
			maxScore: 1.0,
		},

		// Good tier
		{
			name:     "semantic article with class",
			selector: "article.post",
			wantTier: "good",
			minScore: 0.7,
			maxScore: 0.89,
		},
		{
			name:     "ARIA role",
			selector: `[role="listitem"]`,
			wantTier: "good",
			minScore: 0.7,
			maxScore: 0.89,
		},
		{
			name:     "semantic nav",
			selector: "nav",
			wantTier: "good",
			minScore: 0.7,
			maxScore: 0.89,
		},
		{
			name:     "semantic section",
			selector: "section > div",
			wantTier: "good",
			minScore: 0.7,
			maxScore: 0.89,
		},

		// Fair tier
		{
			name:     "class-based selector",
			selector: ".product-card",
			wantTier: "fair",
			minScore: 0.5,
			maxScore: 0.69,
		},
		{
			name:     "class with child",
			selector: ".list > .item",
			wantTier: "fair",
			minScore: 0.5,
			maxScore: 0.69,
		},

		// Fragile tier
		{
			name:     "deeply nested classes",
			selector: ".container .row .col .item",
			wantTier: "fragile",
			minScore: 0.0,
			maxScore: 0.49,
		},
		{
			name:     "nth-child positional",
			selector: "div > span:nth-child(2)",
			wantTier: "fragile",
			minScore: 0.0,
			maxScore: 0.49,
		},
		{
			name:     "tag-only div",
			selector: "div",
			wantTier: "fragile",
			minScore: 0.0,
			maxScore: 0.49,
		},
		{
			name:     "tag-only span",
			selector: "span",
			wantTier: "fragile",
			minScore: 0.0,
			maxScore: 0.49,
		},
		{
			name:     "empty selector",
			selector: "",
			wantTier: "fragile",
			minScore: 0.0,
			maxScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreSelector(tt.selector)
			if got.Tier != tt.wantTier {
				t.Errorf("ScoreSelector(%q).Tier = %q, want %q (score=%.2f, reason=%s)",
					tt.selector, got.Tier, tt.wantTier, got.Score, got.Reason)
			}

			if got.Score < tt.minScore || got.Score > tt.maxScore {
				t.Errorf("ScoreSelector(%q).Score = %.2f, want [%.2f, %.2f] (tier=%s, reason=%s)",
					tt.selector, got.Score, tt.minScore, tt.maxScore, got.Tier, got.Reason)
			}

			if got.Selector != tt.selector {
				t.Errorf("ScoreSelector(%q).Selector = %q, want %q", tt.selector, got.Selector, tt.selector)
			}

			if got.Reason == "" {
				t.Errorf("ScoreSelector(%q).Reason is empty", tt.selector)
			}
		})
	}
}

func TestCombinatorDepth(t *testing.T) {
	tests := []struct {
		sel  string
		want int
	}{
		{"div", 0},
		{"div > span", 1},
		{"div > span > a", 2},
		{".a .b .c .d", 3},
		{".a .b .c .d .e", 4},
		{`[data-x="a b"] > div`, 1},
		{"div + span ~ p", 2},
	}

	for _, tt := range tests {
		t.Run(tt.sel, func(t *testing.T) {
			got := combinatorDepth(tt.sel)
			if got != tt.want {
				t.Errorf("combinatorDepth(%q) = %d, want %d", tt.sel, got, tt.want)
			}
		})
	}
}

func TestScoreRunbookSelectors(t *testing.T) {
	r := &Runbook{
		Version: "1",
		Name:    "test",
		Type:    "extract",
		URL:     "http://example.com",
		WaitFor: `[data-testid="list"]`,
		Items: &ItemSpec{
			Container: `[data-testid="list"]`,
			Fields: map[string]string{
				"title": "h2",
				"link":  "a@href",
				"price": ".price",
			},
		},
		Pagination: &Pagination{
			Strategy:     "click",
			NextSelector: `a[rel="next"]`,
			MaxPages:     5,
		},
	}

	scores := ScoreRunbookSelectors(r)

	// Should have entries for: container, wait_for, field:title, field:link, field:price, pagination:next
	expectedKeys := []string{"container", "wait_for", "field:title", "field:link", "field:price", "pagination:next"}
	for _, key := range expectedKeys {
		if _, ok := scores[key]; !ok {
			t.Errorf("ScoreRunbookSelectors: missing key %q", key)
		}
	}

	// container uses data-*, should be excellent
	if s := scores["container"]; s.Tier != "excellent" {
		t.Errorf("container tier = %q, want excellent (score=%.2f)", s.Tier, s.Score)
	}

	// field:title is tag-only "h2", should be fragile
	if s := scores["field:title"]; s.Tier != "fragile" {
		t.Errorf("field:title tier = %q, want fragile (score=%.2f)", s.Tier, s.Score)
	}

	// field:price is class-based ".price", should be fair
	if s := scores["field:price"]; s.Tier != "fair" {
		t.Errorf("field:price tier = %q, want fair (score=%.2f)", s.Tier, s.Score)
	}
}

func TestIsTagOnly(t *testing.T) {
	tests := []struct {
		sel  string
		want bool
	}{
		{"div", true},
		{"span", true},
		{"h2", true},
		{".class", false},
		{"#id", false},
		{"div.class", false},
		{"div > span", false},
		{"[attr]", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.sel, func(t *testing.T) {
			if got := isTagOnly(tt.sel); got != tt.want {
				t.Errorf("isTagOnly(%q) = %v, want %v", tt.sel, got, tt.want)
			}
		})
	}
}
