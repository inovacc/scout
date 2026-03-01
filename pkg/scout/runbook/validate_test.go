package runbook

import (
	"testing"
)

func TestValidateRunbook_Valid(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "valid", Type: "extract",
		URL:   ts.URL + "/runbook-extract",
		Items: &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2", "link": "a@href"}},
	}

	result, err := ValidateRunbook(b, r)
	if err != nil {
		t.Fatalf("ValidateRunbook failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid=true, got false; errors: %v", result.Errors)
	}
	if result.SampleItems != 3 {
		t.Errorf("sample_items = %d, want 3", result.SampleItems)
	}
	if result.URL != ts.URL+"/runbook-extract" {
		t.Errorf("url = %q, want %q", result.URL, ts.URL+"/runbook-extract")
	}
}

func TestValidateRunbook_Invalid(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "invalid", Type: "extract",
		URL:   ts.URL + "/runbook-extract",
		Items: &ItemSpec{Container: ".nonexistent", Fields: map[string]string{"title": ".fake-selector", "link": "span.missing@data-id"}},
	}

	result, err := ValidateRunbook(b, r)
	if err != nil {
		t.Fatalf("ValidateRunbook failed: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for bad selectors")
	}
	if len(result.Errors) == 0 {
		t.Error("expected validation errors")
	}
	if result.SampleItems != 0 {
		t.Errorf("sample_items = %d, want 0", result.SampleItems)
	}

	// Check that container error is present.
	found := false
	for _, e := range result.Errors {
		if e.Field == "container" && e.Selector == ".nonexistent" {
			found = true
		}
	}
	if !found {
		t.Error("expected error for container selector")
	}
}

func TestValidateRunbook_Automate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "auto", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "click", Selector: "#btn"},
			{Action: "type", Selector: ".no-such-element", Text: "x"},
		},
	}

	result, err := ValidateRunbook(b, r)
	if err != nil {
		t.Fatalf("ValidateRunbook failed: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for missing .no-such-element")
	}

	// #btn should be fine, .no-such-element should fail.
	foundGood := false
	foundBad := false
	for _, e := range result.Errors {
		if e.Selector == "#btn" {
			foundGood = true
		}
		if e.Selector == ".no-such-element" {
			foundBad = true
		}
	}
	if foundGood {
		t.Error("#btn should not be in errors")
	}
	if !foundBad {
		t.Error(".no-such-element should be in errors")
	}
}

func TestValidateRunbook_NilRunbook(t *testing.T) {
	b := newTestBrowser(t)
	_, err := ValidateRunbook(b, nil)
	if err == nil {
		t.Error("expected error for nil runbook")
	}
}

func TestValidateRunbook_NoURL(t *testing.T) {
	b := newTestBrowser(t)
	r := &Runbook{Version: "1", Name: "no-url", Type: "automate", Steps: []Step{{Action: "click", Selector: "#x"}}}
	_, err := ValidateRunbook(b, r)
	if err == nil {
		t.Error("expected error for runbook with no URL")
	}
}

func TestSelectorHealthCheck(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/runbook-extract")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	selectors := map[string]string{
		"items":   ".item",
		"heading": "h2",
		"missing": ".does-not-exist",
		"attr":    "a@href",
		"sibling": "+.total",
	}

	counts := SelectorHealthCheck(page, selectors)

	if counts["items"] != 3 {
		t.Errorf("items count = %d, want 3", counts["items"])
	}
	if counts["heading"] != 3 {
		t.Errorf("heading count = %d, want 3", counts["heading"])
	}
	if counts["missing"] != 0 {
		t.Errorf("missing count = %d, want 0", counts["missing"])
	}
	if counts["attr"] != 3 {
		t.Errorf("attr count = %d, want 3 (should strip @href)", counts["attr"])
	}
	if counts["sibling"] != 1 {
		t.Errorf("sibling count = %d, want 1 (should strip + prefix)", counts["sibling"])
	}
}
