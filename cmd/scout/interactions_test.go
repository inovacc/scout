package main

import "testing"

func TestInteractionsCommandRegistered(t *testing.T) {
	if interactionsOnCmd.Flags().Lookup("dir") == nil {
		t.Fatal("interactions on --dir flag not registered")
	}
	sub := map[string]bool{}
	for _, c := range interactionsCmd.Commands() {
		sub[c.Name()] = true
	}
	for _, want := range []string{"on", "off", "status", "list"} {
		if !sub[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}
