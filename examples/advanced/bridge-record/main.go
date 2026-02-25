// Example: bridge-record
// Records browser interactions via the bridge and outputs a recipe JSON.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	url := "https://quotes.toscrape.com"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	browser, err := scout.New(
		scout.WithHeadless(false),
		scout.WithNoSandbox(),
		scout.WithBridge(),
		scout.WithBridgePort(0),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = browser.Close() }()

	bs := browser.BridgeServer()
	if bs == nil {
		log.Fatal("bridge WebSocket server not started")
	}

	_, err = browser.NewPage(url)
	if err != nil {
		log.Fatal(err)
	}

	// Start recording.
	rec := scout.NewBridgeRecorder(bs)
	rec.Start()

	_, _ = fmt.Fprintf(os.Stderr, "Recording interactions on %s...\nPress Ctrl+C to stop.\n", url)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh

	// Stop and convert to recipe.
	steps := rec.Stop()
	recipe := rec.ToRecipe("Recorded Session", url)

	_, _ = fmt.Fprintf(os.Stderr, "\nRecorded %d steps.\n", len(steps))

	data, err := json.MarshalIndent(recipe, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}
