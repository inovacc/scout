package main

import "testing"

// TestPairingHostDefaultsLoopback guards the fix for the plaintext pairing
// exposure: the pairing listener must default to loopback so the token + certs
// are not broadcast to the network.
func TestPairingHostDefaultsLoopback(t *testing.T) {
	f := serverCmd.Flags().Lookup("pairing-host")
	if f == nil {
		t.Fatal("pairing-host flag not registered")
	}

	if f.DefValue != "127.0.0.1" {
		t.Fatalf("pairing-host default = %q, want 127.0.0.1", f.DefValue)
	}
}
