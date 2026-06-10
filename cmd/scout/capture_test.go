package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/inovacc/scout/pkg/scout/capture"
)

func TestGenerateExtensionKey(t *testing.T) {
	dir := t.TempDir()
	keyValue, extID, err := generateExtensionKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(extID) != 32 {
		t.Fatalf("extID len = %d", len(extID))
	}
	// The PEM must exist and be 0600.
	pemPath := filepath.Join(dir, "extension_key.pem")
	fi, err := os.Stat(pemPath)
	if err != nil {
		t.Fatalf("private key not written: %v", err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %v, want 0600", fi.Mode().Perm())
	}
	// keyValue must be valid base64 of a DER SPKI that re-derives extID.
	der, err := base64.StdEncoding.DecodeString(keyValue)
	if err != nil {
		t.Fatalf("key value not base64: %v", err)
	}
	if capture.ExtensionID(der) != extID {
		t.Fatal("returned extID does not match key value")
	}
}

func TestCaptureCommandsRegistered(t *testing.T) {
	names := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
		for _, sub := range c.Commands() {
			names[c.Name()+" "+sub.Name()] = true
		}
	}
	if !names["capture-host"] {
		t.Error("capture-host not registered")
	}
	if !names["vault capture-key"] {
		t.Error("vault capture-key not registered")
	}
	if !names["vault import-captures"] {
		t.Error("vault import-captures not registered")
	}
}
