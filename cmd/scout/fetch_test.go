package main

import (
	"bytes"
	"testing"
)

func TestFetchCmd_NoURL(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"fetch"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no URL provided")
	}

	want := "scout: --url or positional URL is required"
	if err != nil && err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestFetchCmd_FlagDefaults(t *testing.T) {
	mode, err := fetchCmd.Flags().GetString("mode")
	if err != nil {
		t.Fatalf("mode flag not found: %v", err)
	}

	if mode != "full" {
		t.Errorf("default mode = %q, want %q", mode, "full")
	}

	mainOnly, err := fetchCmd.Flags().GetBool("main-only")
	if err != nil {
		t.Fatalf("main-only flag not found: %v", err)
	}

	if mainOnly {
		t.Error("main-only should default to false")
	}

	includeHTML, err := fetchCmd.Flags().GetBool("include-html")
	if err != nil {
		t.Fatalf("include-html flag not found: %v", err)
	}

	if includeHTML {
		t.Error("include-html should default to false")
	}
}

func TestFetchCmd_FlagRegistration(t *testing.T) {
	flags := []struct {
		name     string
		flagType string
	}{
		{"url", "string"},
		{"mode", "string"},
		{"main-only", "bool"},
		{"include-html", "bool"},
	}

	for _, f := range flags {
		flag := fetchCmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("flag %q not registered", f.name)
			continue
		}

		if flag.Value.Type() != f.flagType {
			t.Errorf("flag %q type = %q, want %q", f.name, flag.Value.Type(), f.flagType)
		}
	}
}

func TestFetchCmd_UsageString(t *testing.T) {
	if fetchCmd.Use != "fetch" {
		t.Errorf("Use = %q, want %q", fetchCmd.Use, "fetch")
	}

	if fetchCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if fetchCmd.Long == "" {
		t.Error("Long description should not be empty")
	}
}
