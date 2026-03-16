package stealth

import (
	"regexp"
	"strings"
	"testing"
)

func TestJSVersion_Format(t *testing.T) {
	// JSVersion should follow semver-like format: vX.Y.Z
	matched, err := regexp.MatchString(`^v\d+\.\d+\.\d+$`, JSVersion)
	if err != nil {
		t.Fatalf("regexp: %v", err)
	}

	if !matched {
		t.Errorf("JSVersion = %q, want semver format vX.Y.Z", JSVersion)
	}
}

func TestJSVersion_KnownValue(t *testing.T) {
	if JSVersion != "v2.7.3" {
		t.Errorf("JSVersion = %q, want %q", JSVersion, "v2.7.3")
	}
}

func TestJS_NotEmpty(t *testing.T) {
	if len(JS) == 0 {
		t.Fatal("JS constant should not be empty")
	}
}

func TestJS_IsIIFE(t *testing.T) {
	// The main stealth JS should be wrapped in an IIFE
	trimmed := strings.TrimSpace(JS)
	if !strings.HasPrefix(trimmed, ";") {
		t.Error("JS should start with semicolon (defensive IIFE)")
	}

	if !strings.Contains(trimmed, "(() => {") && !strings.Contains(trimmed, "(function()") {
		t.Error("JS should contain an IIFE pattern")
	}
}

func TestJS_BalancedBraces(t *testing.T) {
	count := 0
	for _, c := range JS {
		switch c {
		case '{':
			count++
		case '}':
			count--
		}

		if count < 0 {
			t.Fatal("JS has more closing braces than opening braces")
		}
	}

	if count != 0 {
		t.Errorf("JS has unbalanced braces: %d unclosed", count)
	}
}

func TestJS_ParensNotWildlyUnbalanced(t *testing.T) {
	// The main JS may have regex patterns with unescaped parens,
	// so we only check that the imbalance is small (not structural corruption).
	open := strings.Count(JS, "(")
	close := strings.Count(JS, ")")
	diff := open - close
	if diff < 0 {
		diff = -diff
	}

	// Allow small imbalance from regex literals
	if diff > 10 {
		t.Errorf("JS parens imbalance too large: open=%d close=%d diff=%d", open, close, diff)
	}
}

func TestJS_BalancedBrackets(t *testing.T) {
	count := 0
	for _, c := range JS {
		switch c {
		case '[':
			count++
		case ']':
			count--
		}

		if count < 0 {
			t.Fatal("JS has more closing brackets than opening brackets")
		}
	}

	if count != 0 {
		t.Errorf("JS has unbalanced brackets: %d unclosed", count)
	}
}

func TestJS_ContainsCoreEvasions(t *testing.T) {
	// The main stealth JS from extract-stealth-evasions should contain
	// core anti-detection patterns
	tests := []struct {
		name    string
		pattern string
	}{
		{"webdriver", "webdriver"},
		{"chrome_runtime", "chrome"},
		{"navigator", "navigator"},
		{"prototype", "prototype"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(JS, tt.pattern) {
				t.Errorf("JS missing core evasion pattern %q", tt.pattern)
			}
		})
	}
}

func TestJS_NoSyntaxErrorMarkers(t *testing.T) {
	// Check for obvious syntax issues
	tests := []struct {
		name    string
		bad     string
	}{
		{"double_semicolons_in_row", ";;;;\n;;;;"},
		{"undefined_literal_leak", "= undefined;undefined;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(JS, tt.bad) {
				t.Errorf("JS contains suspicious pattern %q", tt.bad)
			}
		})
	}
}

func TestJS_SizeReasonable(t *testing.T) {
	// The stealth JS should be substantial but not absurdly large
	size := len(JS)
	if size < 1000 {
		t.Errorf("JS seems too small (%d bytes), expected substantial evasion script", size)
	}

	if size > 500000 {
		t.Errorf("JS seems too large (%d bytes), possible corruption", size)
	}
}

func TestExtraJS_SizeReasonable(t *testing.T) {
	size := len(ExtraJS)
	if size < 1000 {
		t.Errorf("ExtraJS seems too small (%d bytes)", size)
	}

	if size > 100000 {
		t.Errorf("ExtraJS seems too large (%d bytes)", size)
	}
}

func TestExtraJS_IIFECount(t *testing.T) {
	// ExtraJS has 17 numbered evasion sections, most wrapped in IIFEs
	count := strings.Count(ExtraJS, "(function()")
	if count < 10 {
		t.Errorf("ExtraJS has %d IIFEs, expected at least 10 for the 17 evasion sections", count)
	}
}

func TestExtraJS_NoDebugStatements(t *testing.T) {
	// Production evasion script should not contain debug logging
	debugPatterns := []string{
		"console.log(",
		"console.debug(",
		"console.info(",
		"debugger;",
		"alert(",
	}

	for _, p := range debugPatterns {
		if strings.Contains(ExtraJS, p) {
			t.Errorf("ExtraJS contains debug statement %q", p)
		}
	}
}

func TestJS_NoDebugStatements(t *testing.T) {
	debugPatterns := []string{
		"console.log(",
		"console.debug(",
		"debugger;",
		"alert(",
	}

	for _, p := range debugPatterns {
		if strings.Contains(JS, p) {
			t.Errorf("JS contains debug statement %q", p)
		}
	}
}
