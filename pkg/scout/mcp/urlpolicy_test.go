package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/urlpolicy"
)

func TestCheckURLBlocksInternalByDefault(t *testing.T) {
	s := &mcpState{policy: urlpolicy.FromEnv()} // no env → block-by-default

	for _, u := range []string{"file:///etc/passwd", "http://127.0.0.1", "http://169.254.169.254"} {
		if err := s.checkURL(context.Background(), u); err == nil {
			t.Errorf("checkURL(%q) = nil, want blocked", u)
		} else if !strings.HasPrefix(err.Error(), "blocked") {
			t.Errorf("checkURL(%q) error = %q, want blocked-prefixed", u, err)
		}
	}

	if err := s.checkURL(context.Background(), "https://example.com"); err != nil {
		t.Errorf("checkURL(public) = %v, want allowed", err)
	}
}

func TestCheckURLAllowLocalEnv(t *testing.T) {
	t.Setenv("SCOUT_ALLOW_LOCAL_TARGETS", "1")
	s := &mcpState{policy: urlpolicy.FromEnv()}
	if err := s.checkURL(context.Background(), "http://127.0.0.1"); err != nil {
		t.Errorf("with AllowLocal, checkURL(loopback) = %v, want allowed", err)
	}
}
