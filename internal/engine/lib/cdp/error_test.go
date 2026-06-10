package cdp

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorError(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "code and message",
			err:  &Error{Code: -32000, Message: "boom"},
			want: "{-32000 boom }",
		},
		{
			name: "with data",
			err:  &Error{Code: 1, Message: "msg", Data: "extra"},
			want: "{1 msg extra}",
		},
		{
			name: "zero value",
			err:  &Error{},
			want: "{0  }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorIs(t *testing.T) {
	base := &Error{Code: -32000, Message: "Cannot find context with specified id"}

	tests := []struct {
		name   string
		err    *Error
		target error
		want   bool
	}{
		{
			name:   "identical fields match",
			err:    &Error{Code: -32000, Message: "Cannot find context with specified id"},
			target: base,
			want:   true,
		},
		{
			name:   "different code does not match",
			err:    &Error{Code: -32001, Message: "Cannot find context with specified id"},
			target: base,
			want:   false,
		},
		{
			name:   "different message does not match",
			err:    &Error{Code: -32000, Message: "different"},
			target: base,
			want:   false,
		},
		{
			name:   "different data does not match",
			err:    &Error{Code: -32000, Message: "Cannot find context with specified id", Data: "x"},
			target: base,
			want:   false,
		},
		{
			name:   "non-cdp error target does not match",
			err:    &Error{Code: -32000, Message: "Cannot find context with specified id"},
			target: errors.New("plain"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Is(tt.target); got != tt.want {
				t.Fatalf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorIsViaErrorsIs verifies the predeclared sentinel errors are matchable
// through the stdlib errors.Is using the custom Is method, including when wrapped.
func TestErrorIsViaErrorsIs(t *testing.T) {
	tests := []struct {
		name     string
		got      *Error
		sentinel *Error
		match    bool
	}{
		{
			name:     "ctx not found matches sentinel",
			got:      &Error{Code: -32000, Message: "Cannot find context with specified id"},
			sentinel: ErrCtxNotFound,
			match:    true,
		},
		{
			name:     "session not found matches sentinel",
			got:      &Error{Code: -32001, Message: "Session with given id not found."},
			sentinel: ErrSessionNotFound,
			match:    true,
		},
		{
			name:     "ctx not found does not equal session not found",
			got:      ErrCtxNotFound,
			sentinel: ErrSessionNotFound,
			match:    false,
		},
		{
			name:     "obj not found matches sentinel",
			got:      &Error{Code: -32000, Message: "Could not find object with given id"},
			sentinel: ErrObjNotFound,
			match:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := fmt.Errorf("scout: cdp: %w", tt.got)
			if got := errors.Is(wrapped, tt.sentinel); got != tt.match {
				t.Fatalf("errors.Is(wrapped, sentinel) = %v, want %v", got, tt.match)
			}
		})
	}
}

func TestSentinelErrorsDistinct(t *testing.T) {
	// ErrSearchSessionNotFound, ErrCtxDestroyed, ErrObjNotFound, ErrNodeNotFoundAtPos,
	// ErrNotAttachedToActivePage all share code -32000 but carry distinct messages,
	// so they must not be considered equal to each other.
	all := []*Error{
		ErrSearchSessionNotFound,
		ErrCtxDestroyed,
		ErrObjNotFound,
		ErrNodeNotFoundAtPos,
		ErrNotAttachedToActivePage,
	}

	for i := range all {
		for j := range all {
			if i == j {
				continue
			}
			if all[i].Is(all[j]) {
				t.Fatalf("sentinel %q must not equal %q", all[i].Message, all[j].Message)
			}
		}
	}
}
