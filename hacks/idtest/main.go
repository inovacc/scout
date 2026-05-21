// idtest creates a reusable browser, prints its session ID, then exits
// WITHOUT closing — the reusable session dir stays so we can inspect it.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	br, err := scout.New(
		scout.WithBrowser(scout.BrowserChrome),
		scout.WithHeadless(true),
		scout.WithReusableSession(),
		scout.WithReusableLifetime(1*time.Minute),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "New:", err)
		os.Exit(1)
	}

	fmt.Println("SESSION_ID:", br.SessionID())

	_, _ = br.NewPage("about:blank")
	time.Sleep(1 * time.Second)
	// Do NOT call br.Close(); reusable session must persist.
}
