package main

import (
	"strings"
	"testing"
)

func TestVerifyPluginChecksum(t *testing.T) {
	data := []byte("hello plugin archive")
	good := sha256Hex(data)

	if err := verifyPluginChecksum(data, good); err != nil {
		t.Errorf("matching checksum should pass: %v", err)
	}
	if err := verifyPluginChecksum(data, "  "+strings.ToUpper(good)+"  "); err != nil {
		t.Errorf("checksum compare should be trimmed + case-insensitive: %v", err)
	}
	if err := verifyPluginChecksum(data, "deadbeef"); err == nil {
		t.Error("mismatched checksum must fail")
	}
	if err := verifyPluginChecksum(data, ""); err != nil {
		t.Errorf("empty checksum is allowed (caller warns): %v", err)
	}
}
