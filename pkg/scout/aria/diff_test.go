package aria_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

func readGolden(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	return strings.TrimRight(string(raw), "\n")
}

func TestDiff_Add(t *testing.T) {
	t.Parallel()
	before := loadFixture(t, "diff_add_before")
	after := loadFixture(t, "diff_add_after")
	got := aria.Diff(before, after).String()
	want := readGolden(t, "diff_add_summary.txt")
	if got != want {
		t.Errorf("summary mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDiff_Remove(t *testing.T) {
	t.Parallel()
	before := loadFixture(t, "diff_remove_before")
	after := loadFixture(t, "diff_remove_after")
	got := aria.Diff(before, after).String()
	want := readGolden(t, "diff_remove_summary.txt")
	if got != want {
		t.Errorf("summary mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDiff_Noop(t *testing.T) {
	t.Parallel()
	before := loadFixture(t, "diff_noop_before")
	after := loadFixture(t, "diff_noop_after")
	got := aria.Diff(before, after).String()
	want := readGolden(t, "diff_noop_summary.txt")
	if got != want {
		t.Errorf("summary mismatch\ngot:  %q\nwant: %q", got, want)
	}
}
