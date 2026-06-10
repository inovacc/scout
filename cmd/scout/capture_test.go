package main

import "testing"

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
