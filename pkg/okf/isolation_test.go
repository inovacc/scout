package okf_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestImportIsolation verifies that pkg/okf has zero dependencies on
// scout-internal packages (github.com/inovacc/scout/internal/...) and on the
// public scout facade (github.com/inovacc/scout/pkg/scout...).
func TestImportIsolation(t *testing.T) {
	t.Parallel()

	out, err := exec.Command("go", "list", "-deps", "github.com/inovacc/scout/pkg/okf").Output()
	if err != nil {
		t.Fatalf("go list -deps failed: %v", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "github.com/inovacc/scout/internal/") {
			t.Errorf("forbidden internal dep: %s", line)
		}
		if strings.Contains(line, "github.com/inovacc/scout/pkg/scout") {
			t.Errorf("forbidden scout facade dep: %s", line)
		}
	}
}
