package vault

import (
	"bytes"
	"testing"
)

func TestLockedBufferZeroOnClose(t *testing.T) {
	secret := []byte("hunter2-super-secret")
	lb := NewLockedBuffer(append([]byte(nil), secret...))

	if !bytes.Equal(lb.Bytes(), secret) {
		t.Fatalf("Bytes() = %q, want %q", lb.Bytes(), secret)
	}
	if lb.Len() != len(secret) {
		t.Fatalf("Len() = %d, want %d", lb.Len(), len(secret))
	}

	backing := lb.Bytes() // same backing array
	if err := lb.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	for i, b := range backing {
		if b != 0 {
			t.Fatalf("byte %d = %d after Close, want 0", i, b)
		}
	}
	if lb.Len() != 0 {
		t.Fatalf("Len() = %d after Close, want 0", lb.Len())
	}
}

func TestLockedBufferEqualConstantTime(t *testing.T) {
	lb := NewLockedBuffer([]byte("token-abc"))
	defer func() { _ = lb.Close() }()

	if !lb.Equal([]byte("token-abc")) {
		t.Fatal("Equal returned false for matching token")
	}
	if lb.Equal([]byte("token-xyz")) {
		t.Fatal("Equal returned true for non-matching token")
	}
	if lb.Equal([]byte("token-abc-longer")) {
		t.Fatal("Equal returned true for different-length token")
	}
}

func TestNewLockedBufferEmpty(t *testing.T) {
	lb := NewLockedBuffer(nil)
	if lb.Len() != 0 {
		t.Fatalf("Len() = %d, want 0 for nil input", lb.Len())
	}
	if lb.locked {
		t.Fatal("empty buffer should not be marked locked")
	}
	lb.Zero() // must not panic
	if err := lb.Close(); err != nil {
		t.Fatalf("Close() on empty buffer error = %v", err)
	}
}

func TestLockedBufferDoubleClose(t *testing.T) {
	lb := NewLockedBuffer([]byte("secret"))
	if err := lb.Close(); err != nil {
		t.Fatalf("first Close error = %v", err)
	}
	if err := lb.Close(); err != nil { // must be safe and idempotent
		t.Fatalf("second Close error = %v", err)
	}
	if lb.Len() != 0 {
		t.Fatalf("Len() = %d after double close, want 0", lb.Len())
	}
}
