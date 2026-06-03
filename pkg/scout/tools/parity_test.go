package tools

import "testing"

// verbRegistry is the authoritative list of browser verbs and where each is
// exposed. Update this when adding a verb; CI fails if a verb is unmapped.
var verbRegistry = []struct {
	Verb     string
	InREPL   bool
	InMCP    bool
	WaiveMCP string // reason MCP exposure is intentionally skipped, or ""
}{
	{"Navigate", true, true, ""},
	{"Back", true, true, ""},
	{"Forward", true, true, ""},
	{"Reload", true, true, ""},
	{"Wait", true, true, ""},
	{"Click", true, true, ""},
	{"Type", true, true, ""},
	{"Extract", true, true, ""},
	{"Eval", true, true, ""},
	{"HTML", true, true, ""},
	{"Markdown", true, true, ""},
	{"Cookies", true, true, ""},
	{"URL", true, true, ""},
	{"Title", true, true, ""},
	{"Screenshot", true, true, ""},
	{"Snapshot", false, true, "REPL has no snapshot command"},
	{"PDF", false, true, "REPL has no pdf command"},
	{"Tabs", true, true, ""},
	{"NewTab", true, true, ""},
}

func TestVerbParity(t *testing.T) {
	for _, v := range verbRegistry {
		if !v.InREPL && !v.InMCP {
			t.Errorf("verb %s exposed nowhere", v.Verb)
		}
		if !v.InMCP && v.WaiveMCP == "" {
			t.Errorf("verb %s missing from MCP without a waiver reason", v.Verb)
		}
	}
}
