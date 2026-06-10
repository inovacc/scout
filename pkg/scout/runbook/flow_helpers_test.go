package runbook

import "testing"

func TestStringVal(t *testing.T) {
	m := map[string]any{
		"s":    "hello",
		"n":    42.0,
		"b":    true,
		"nilv": nil,
	}

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"present string", "s", "hello"},
		{"present non-string (number)", "n", ""},
		{"present non-string (bool)", "b", ""},
		{"present nil value", "nilv", ""},
		{"absent key", "missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringVal(m, tt.key); got != tt.want {
				t.Errorf("stringVal(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestBoolVal(t *testing.T) {
	m := map[string]any{
		"t":   true,
		"f":   false,
		"s":   "true",
		"n":   1.0,
		"nil": nil,
	}

	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"present true", "t", true},
		{"present false", "f", false},
		{"present non-bool string", "s", false},
		{"present non-bool number", "n", false},
		{"present nil value", "nil", false},
		{"absent key", "missing", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := boolVal(m, tt.key); got != tt.want {
				t.Errorf("boolVal(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// TestGenerateFlowRunbook_LoginSteps pins the exact step emission for a login
// page, including the username/password field mapping and the submit click.
func TestGenerateFlowRunbook_LoginSteps(t *testing.T) {
	steps := []FlowStep{
		{
			URL:      "https://example.com/login",
			PageType: "login",
			IsLogin:  true,
			Forms: []FormInfo{
				{
					Selector:    "form#login",
					HasPassword: true,
					Fields:      []string{"email", "password"},
				},
			},
		},
	}

	r, err := GenerateFlowRunbook(steps, "login-flow")
	if err != nil {
		t.Fatalf("GenerateFlowRunbook() error: %v", err)
	}

	if r.Type != "automate" {
		t.Fatalf("type = %q, want automate", r.Type)
	}

	// Expect: navigate, type(email -> {{username}}), type(password -> {{password}}), click submit.
	if len(r.Steps) != 4 {
		t.Fatalf("step count = %d, want 4: %+v", len(r.Steps), r.Steps)
	}

	if r.Steps[0].Action != "navigate" {
		t.Errorf("step 0 = %+v, want navigate", r.Steps[0])
	}

	var sawUsername, sawPassword, sawSubmit bool

	for _, s := range r.Steps {
		switch {
		case s.Action == "type" && s.Text == "{{username}}":
			sawUsername = true
		case s.Action == "type" && s.Text == "{{password}}":
			sawPassword = true
		case s.Action == "click":
			sawSubmit = true
			if s.Selector == "" {
				t.Error("submit click step has empty selector")
			}
		}
	}

	if !sawUsername {
		t.Error("expected a type step mapping the email field to {{username}}")
	}

	if !sawPassword {
		t.Error("expected a type step mapping the password field to {{password}}")
	}

	if !sawSubmit {
		t.Error("expected a submit click step")
	}
}

// TestGenerateFlowRunbook_SearchAndListing covers the search and listing
// branches of the step generator in a single multi-step flow.
func TestGenerateFlowRunbook_SearchAndListing(t *testing.T) {
	steps := []FlowStep{
		{
			URL:      "https://example.com/search",
			PageType: "search",
			IsSearch: true,
			Forms: []FormInfo{
				{Selector: "form#q", HasSearch: true, Fields: []string{"q"}},
			},
		},
		{
			URL:      "https://example.com/results",
			PageType: "listing",
		},
	}

	r, err := GenerateFlowRunbook(steps, "search-flow")
	if err != nil {
		t.Fatalf("GenerateFlowRunbook() error: %v", err)
	}

	var sawQueryType, sawExtract bool

	for _, s := range r.Steps {
		if s.Action == "type" && s.Text == "{{query}}" {
			sawQueryType = true
		}

		if s.Action == "extract" && s.As == "items" {
			sawExtract = true
		}
	}

	if !sawQueryType {
		t.Errorf("expected a search type step with {{query}}, steps: %+v", r.Steps)
	}

	if !sawExtract {
		t.Errorf("expected an extract step for the listing page, steps: %+v", r.Steps)
	}
}

// TestGenerateFlowRunbook_NoActionableSteps ensures the "could not generate"
// error fires when steps produce only no-op navigations... actually navigate
// steps are always emitted, so an unknown-only flow still yields navigate steps.
// This test pins that an unknown page still generates a navigate-only runbook.
func TestGenerateFlowRunbook_UnknownOnly(t *testing.T) {
	steps := []FlowStep{
		{URL: "https://example.com/a", PageType: "unknown"},
		{URL: "https://example.com/b", PageType: "unknown"},
	}

	r, err := GenerateFlowRunbook(steps, "unknown-flow")
	if err != nil {
		t.Fatalf("GenerateFlowRunbook() error: %v", err)
	}

	// Each unknown step still emits exactly one navigate step.
	if len(r.Steps) != 2 {
		t.Fatalf("step count = %d, want 2 navigate steps: %+v", len(r.Steps), r.Steps)
	}

	for i, s := range r.Steps {
		if s.Action != "navigate" {
			t.Errorf("step %d = %+v, want navigate", i, s)
		}
	}
}
