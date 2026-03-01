package runbook

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestPrompt_WithDefault(t *testing.T) {
	input := strings.NewReader("\n")
	var output bytes.Buffer
	scanner := bufio.NewScanner(input)

	result := prompt(&output, scanner, "Enter name", "default-value")

	if result != "default-value" {
		t.Errorf("expected %q, got %q", "default-value", result)
	}
	if !strings.Contains(output.String(), "[default-value]") {
		t.Errorf("output should contain default value hint, got %q", output.String())
	}
}

func TestPrompt_WithInput(t *testing.T) {
	input := strings.NewReader("custom-value\n")
	var output bytes.Buffer
	scanner := bufio.NewScanner(input)

	result := prompt(&output, scanner, "Enter name", "default-value")

	if result != "custom-value" {
		t.Errorf("expected %q, got %q", "custom-value", result)
	}
}

func TestPrompt_NoDefault(t *testing.T) {
	input := strings.NewReader("\n")
	var output bytes.Buffer
	scanner := bufio.NewScanner(input)

	result := prompt(&output, scanner, "Enter name", "")

	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
	if strings.Contains(output.String(), "[") {
		t.Errorf("output should not contain brackets for empty default, got %q", output.String())
	}
}

func TestInteractiveCreate_NilBrowser(t *testing.T) {
	_, err := InteractiveCreate(InteractiveConfig{
		URL:    "http://example.com",
		Writer: &bytes.Buffer{},
		Reader: strings.NewReader(""),
	})
	if err == nil || !strings.Contains(err.Error(), "nil browser") {
		t.Errorf("expected nil browser error, got %v", err)
	}
}

func TestInteractiveCreate_EmptyURL(t *testing.T) {
	_, err := InteractiveCreate(InteractiveConfig{
		Writer: &bytes.Buffer{},
		Reader: strings.NewReader(""),
	})
	// Browser is nil too, so nil browser error comes first.
	if err == nil {
		t.Error("expected error, got nil")
		return
	}
}

func TestInteractiveCreate_NilWriter(t *testing.T) {
	_, err := InteractiveCreate(InteractiveConfig{
		URL:    "http://example.com",
		Reader: strings.NewReader(""),
	})
	if err == nil || !strings.Contains(err.Error(), "nil browser") {
		t.Errorf("expected nil browser error, got %v", err)
	}
}
