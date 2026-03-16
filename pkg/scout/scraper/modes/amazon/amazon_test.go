package amazon

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestAmazonMode_Name(t *testing.T) {
	m := &AmazonMode{}
	if got := m.Name(); got != "amazon" {
		t.Errorf("Name() = %q, want %q", got, "amazon")
	}
}

func TestAmazonMode_Description(t *testing.T) {
	m := &AmazonMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestAmazonMode_AuthProvider(t *testing.T) {
	m := &AmazonMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "amazon" {
		t.Errorf("AuthProvider().Name() = %q", p.Name())
	}
}

// --- amazonProvider tests ---

func TestAmazonProvider_LoginURL(t *testing.T) {
	p := &amazonProvider{}
	if got := p.LoginURL(); got != "https://www.amazon.com/ap/signin" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &amazonProvider{}
	if err := p.ValidateSession(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateSession_ValidSessionID(t *testing.T) {
	p := &amazonProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "session-id", Value: "123-456-789"}}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_CaseInsensitive(t *testing.T) {
	p := &amazonProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "Session-Id", Value: "abc"}}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v, expected case-insensitive match", err)
	}
}

func TestValidateSession_NoCookie(t *testing.T) {
	p := &amazonProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "other", Value: "val"}}}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*scraper.AuthError); !ok {
		t.Errorf("expected *scraper.AuthError, got %T", err)
	}
}

func TestValidateSession_EmptyValue(t *testing.T) {
	p := &amazonProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "session-id", Value: ""}}}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for empty session-id value")
	}
}

// --- isASIN tests ---

func TestIsASIN(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"B08N5WRWNW", true},
		{"0123456789", true},
		{"B08N5wrwnw", false}, // lowercase
		{"B08N5WRWN", false},  // too short
		{"B08N5WRWNWX", false}, // too long
		{"", false},
		{"  B08N5WRWNW  ", false}, // untrimmed but len>10 overall
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isASIN(tt.input); got != tt.want {
				t.Errorf("isASIN(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Note: isASIN trims input, so test with spaces
func TestIsASIN_WithSpaces(t *testing.T) {
	// After trimming "  B08N5WRWNW  " becomes "B08N5WRWNW" which is 10 chars and all uppercase
	if got := isASIN("  B08N5WRWNW  "); got != true {
		t.Errorf("isASIN with spaces = %v, want true", got)
	}
}

// --- buildTargetSet tests ---

func TestBuildTargetSet_Empty(t *testing.T) {
	if set := buildTargetSet(nil); set != nil {
		t.Errorf("expected nil, got %v", set)
	}
}

func TestBuildTargetSet_Normalized(t *testing.T) {
	set := buildTargetSet([]string{"  B08N5WRWNW  ", "laptop"})
	if _, ok := set["b08n5wrwnw"]; !ok {
		t.Error("expected trimmed+lowered ASIN")
	}
	if _, ok := set["laptop"]; !ok {
		t.Error("expected lowered search query")
	}
}
