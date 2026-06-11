package amazon

import (
	"math"
	"testing"
)

// --- parsePrice tests ---

// floatEqual compares two float64 values within a small epsilon to avoid
// binary floating-point representation mismatches.
func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  float64
	}{
		{name: "simple dollar", input: "$19.99", want: 19.99},
		{name: "leading symbol no decimal", input: "$20", want: 20},
		{name: "rupee with thousands separator", input: "₹1,299.00", want: 1299.00},
		{name: "euro with comma stripped", input: "€1,234.56", want: 1234.56},
		{name: "plain integer", input: "42", want: 42},
		{name: "plain decimal", input: "3.50", want: 3.50},
		{name: "surrounding whitespace", input: "  $7.25  ", want: 7.25},
		{name: "trailing currency code", input: "99.95 USD", want: 99.95},
		{name: "price with label prefix", input: "Price: $12.34", want: 12.34},
		{name: "leading zero kept", input: "$0.99", want: 0.99},
		{name: "large value with multiple separators", input: "$1,000,000.00", want: 1000000.00},
		{name: "pound sterling", input: "£45.00", want: 45.00},
		{name: "no fractional part with symbol", input: "USD 250", want: 250},
		{name: "newline and tabs around value", input: "\t$8.88\n", want: 8.88},
		{name: "whole-only with trailing dot kept", input: "$15.", want: 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePrice(tt.input)
			if !floatEqual(got, tt.want) {
				t.Errorf("parsePrice(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestParsePrice_ZeroCases covers inputs that must yield 0 because they contain
// no parseable numeric content after currency/letter stripping.
func TestParsePrice_ZeroCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "whitespace only", input: "   "},
		{name: "letters only", input: "USD"},
		{name: "currency symbol only", input: "$"},
		{name: "symbols only", input: "$,."},
		{name: "no digits with words", input: "Free shipping"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePrice(tt.input); !floatEqual(got, 0) {
				t.Errorf("parsePrice(%q) = %v, want 0", tt.input, got)
			}
		})
	}
}

// TestParsePrice_MultipleDotsUnparseable verifies that when stripping produces a
// string with more than one decimal point (e.g. an integer that already used "."
// as a thousands separator), strconv.ParseFloat fails and parsePrice returns 0.
func TestParsePrice_MultipleDotsUnparseable(t *testing.T) {
	// "1.234.567" keeps both dots -> "1.234.567" is not a valid float -> 0.
	if got := parsePrice("€1.234.567"); !floatEqual(got, 0) {
		t.Errorf("parsePrice with multiple dots = %v, want 0", got)
	}
}
