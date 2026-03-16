package gmaps

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestGMapsMode_Name(t *testing.T) {
	m := &GMapsMode{}
	if got := m.Name(); got != "gmaps" {
		t.Errorf("Name() = %q, want %q", got, "gmaps")
	}
}

func TestGMapsMode_Description(t *testing.T) {
	m := &GMapsMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestGMapsMode_AuthProvider(t *testing.T) {
	m := &GMapsMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "gmaps" {
		t.Errorf("AuthProvider().Name() = %q", p.Name())
	}
}

// --- gmapsProvider tests ---

func TestGMapsProvider_LoginURL(t *testing.T) {
	p := &gmapsProvider{}
	if got := p.LoginURL(); got != "https://accounts.google.com/" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &gmapsProvider{}
	if err := p.ValidateSession(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateSession_ValidSID(t *testing.T) {
	p := &gmapsProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "SID", Value: "abc"}}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidNID(t *testing.T) {
	p := &gmapsProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "NID", Value: "xyz"}}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_NoCookies(t *testing.T) {
	p := &gmapsProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "other", Value: "val"}}}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*scraper.AuthError); !ok {
		t.Errorf("expected *scraper.AuthError, got %T", err)
	}
}

// --- buildTargetSet tests ---

func TestBuildTargetSet_Empty(t *testing.T) {
	if set := buildTargetSet(nil); set != nil {
		t.Errorf("expected nil, got %v", set)
	}
}

func TestBuildTargetSet_Normalized(t *testing.T) {
	set := buildTargetSet([]string{"  Coffee Shop  "})
	if _, ok := set["coffee shop"]; !ok {
		t.Error("expected trimmed+lowered key")
	}
}

// --- encodeSearchQuery tests ---

func TestEncodeSearchQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"coffee shop", "coffee+shop"},
		{"  pizza  ", "pizza"},
		{"singleword", "singleword"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := encodeSearchQuery(tt.input); got != tt.want {
				t.Errorf("encodeSearchQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseBusinessProfile tests ---

func TestParseBusinessProfile_WithNameAndAddress(t *testing.T) {
	body := `{"name":"Coffee House","formatted_address":"123 Main St","rating":4.5}`
	result := parseBusinessProfile(body)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != scraper.ResultProfile {
		t.Errorf("Type = %q", result.Type)
	}
	if result.Source != "gmaps" {
		t.Errorf("Source = %q", result.Source)
	}
}

func TestParseBusinessProfile_NoNameOrAddress(t *testing.T) {
	body := `{"something": "else"}`
	result := parseBusinessProfile(body)
	if result != nil {
		t.Errorf("expected nil for no name/address, got %v", result)
	}
}

// --- parseMapsPLACES tests ---

func TestParseMapsPLACES_InvalidJSON(t *testing.T) {
	results := parseMapsPLACES("not json", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseMapsPLACES_EmptyJSON(t *testing.T) {
	results := parseMapsPLACES("{}", nil)
	if results == nil {
		// OK: no business profile, no reviews, no photos
	}
}

// --- parseHijackEvent tests ---

func TestParseHijackEvent_NonResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventRequest}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseHijackEvent_EmptyBody(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://google.com/maps/preview/search", Body: ""},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseHijackEvent_UnknownURL(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://example.com/other", Body: "{}"},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}
