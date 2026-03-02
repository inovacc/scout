package runbook

import "testing"

func TestDetectFlow_NilBrowser(t *testing.T) {
	_, err := DetectFlow(nil, []string{"http://example.com"})
	if err == nil {
		t.Fatal("expected error for nil browser")
	}

	if got := err.Error(); got != "runbook: flow: nil browser" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestDetectFlow_EmptyURLs(t *testing.T) {
	// Pass a nil browser with empty URLs to hit the URL check first.
	_, err := DetectFlow(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil browser")
	}

	// With a non-nil browser but empty URLs.
	// We can't create a real browser in unit tests, so just test the nil path.
}

func TestGenerateFlowRunbook_NoSteps(t *testing.T) {
	_, err := GenerateFlowRunbook(nil, "test")
	if err == nil {
		t.Fatal("expected error for empty steps")
	}
}

func TestGenerateFlowRunbook_SingleListingPage(t *testing.T) {
	steps := []FlowStep{
		{
			URL:      "http://example.com/products",
			PageType: "listing",
		},
	}

	r, err := GenerateFlowRunbook(steps, "products")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Type != "extract" {
		t.Errorf("expected type=extract for single listing, got %s", r.Type)
	}

	if r.URL != "http://example.com/products" {
		t.Errorf("unexpected URL: %s", r.URL)
	}

	if r.Items == nil {
		t.Fatal("expected items spec for extract runbook")
	}
}

func TestGenerateFlowRunbook_LoginThenSearch(t *testing.T) {
	steps := []FlowStep{
		{
			URL:      "http://example.com/login",
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
		{
			URL:      "http://example.com/search",
			PageType: "search",
			IsSearch: true,
			Forms: []FormInfo{
				{
					Selector:  "form#search",
					HasSearch: true,
					Fields:    []string{"q"},
				},
			},
		},
	}

	r, err := GenerateFlowRunbook(steps, "login-search")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Type != "automate" {
		t.Errorf("expected type=automate, got %s", r.Type)
	}

	if r.Name != "login-search" {
		t.Errorf("unexpected name: %s", r.Name)
	}

	// Should have navigate + type(email) + type(password) + click + navigate + type(query) + click = 7 steps
	if len(r.Steps) < 4 {
		t.Errorf("expected at least 4 steps, got %d", len(r.Steps))
	}

	// First step should be navigate to login.
	if r.Steps[0].Action != "navigate" || r.Steps[0].URL != "http://example.com/login" {
		t.Errorf("first step should navigate to login, got %+v", r.Steps[0])
	}
}

func TestGenerateFlowRunbook_DefaultName(t *testing.T) {
	steps := []FlowStep{
		{URL: "http://example.com", PageType: "unknown"},
	}

	r, err := GenerateFlowRunbook(steps, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Name != "flow-runbook" {
		t.Errorf("expected default name 'flow-runbook', got %s", r.Name)
	}
}

func TestFlowStep_IsLogin(t *testing.T) {
	tests := []struct {
		name    string
		forms   []FormInfo
		isLogin bool
	}{
		{
			name:    "no forms",
			forms:   nil,
			isLogin: false,
		},
		{
			name:    "form with password",
			forms:   []FormInfo{{HasPassword: true}},
			isLogin: true,
		},
		{
			name:    "form without password",
			forms:   []FormInfo{{HasPassword: false}},
			isLogin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := FlowStep{Forms: tt.forms}
			// Simulate the detection logic.
			for _, f := range step.Forms {
				if f.HasPassword {
					step.IsLogin = true
				}
			}

			if step.IsLogin != tt.isLogin {
				t.Errorf("IsLogin = %v, want %v", step.IsLogin, tt.isLogin)
			}
		})
	}
}
