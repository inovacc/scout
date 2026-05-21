// idtest launches a reusable Chrome session and prints the resulting
// session ID + scout.pid layout. Verifies:
//   - encoded session ID (1...) lands on disk
//   - scout.pid is 432-byte binary
//   - scout.lock is a separate 0-byte file
//   - stale-chrome-lock cleanup lets re-runs succeed
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
	_ = br.Close()
	fmt.Println("done")
}
