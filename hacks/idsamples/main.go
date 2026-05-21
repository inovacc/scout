package main

import (
	"fmt"

	"github.com/inovacc/scout/pkg/id"
)

func main() {
	cases := []struct {
		name  string
		attrs id.Attrs
	}{
		{"chrome headless ephemeral", id.Attrs{Browser: "chrome", Headless: true}},
		{"brave visible persistent stealth", id.Attrs{Browser: "brave", Reusable: true, Stealth: true}},
		{"edge headless persistent bridge vpn", id.Attrs{Browser: "edge", Headless: true, Reusable: true, Bridge: true, VPN: true}},
		{"electron visible ephemeral", id.Attrs{Browser: "electron"}},
		{"chromium full", id.Attrs{Browser: "chromium", Headless: true, Stealth: true, Bridge: true, VPN: true}},
	}
	for _, c := range cases {
		s, _ := id.New(c.attrs)
		fmt.Printf("%-42s %s\n", c.name+":", s)
	}
}
