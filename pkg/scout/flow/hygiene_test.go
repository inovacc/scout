package flow

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFlowSpecNeverEmbedsSecretValue(t *testing.T) {
	f := &FlowSpec{
		Version: "1", Name: "h", Auth: &AuthRef{Profile: "p-1"},
		Steps: []FlowStep{{ID: "a", Request: Request{Method: "POST", URL: "https://x",
			Headers: map[string]string{"Authorization": "Bearer ${secret.TOKEN}"},
			JSON:    map[string]any{"pw": "${secret.PASSWORD}"}}}},
	}
	out, err := yaml.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "${secret.TOKEN}") || !strings.Contains(s, "p-1") {
		t.Fatalf("expected refs preserved: %s", s)
	}
	for _, leak := range []string{"hunter2", "sk-live", "Bearer eyJ"} {
		if strings.Contains(s, leak) {
			t.Fatalf("spec leaked a secret-like value: %q", leak)
		}
	}
}
