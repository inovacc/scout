// idtest exercises CleanStaleSessions + StartCleanupRetrier. Runs for
// 75 seconds so the 60-second retry tick fires once.
package main

import (
	"fmt"
	"time"

	"github.com/inovacc/scout/internal/engine/session"
)

func main() {
	n, err := session.CleanStaleSessions()
	fmt.Printf("CleanStaleSessions: cleaned=%d err=%v\n", n, err)
	fmt.Printf("PendingCleanupCount after first sweep: %d\n", session.PendingCleanupCount())

	done := make(chan struct{})
	session.StartCleanupRetrier(done)
	defer close(done)

	for i := 0; i < 5; i++ {
		time.Sleep(15 * time.Second)
		fmt.Printf("[t=%2ds] pending=%d\n", (i+1)*15, session.PendingCleanupCount())
	}
}
