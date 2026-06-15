package main

import (
	"errors"
	"testing"
)

func TestIsNavigationError(t *testing.T) {
	nav := []error{
		errors.New("scout: eval: {-32000 Inspected target navigated or closed }"),
		errors.New("Execution context was destroyed."),
		errors.New("Cannot find context with specified id"),
		errors.New("the execution context was destroyed"),
	}
	for _, e := range nav {
		if !isNavigationError(e) {
			t.Errorf("expected navigation error for %q", e)
		}
	}

	notNav := []error{
		nil,
		errors.New("TypeError: foo is not a function"),
		errors.New("scout: eval: SyntaxError: Unexpected token"),
		errors.New("connection refused"),
	}
	for _, e := range notNav {
		if isNavigationError(e) {
			t.Errorf("did not expect navigation error for %v", e)
		}
	}
}
