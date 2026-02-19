// Example: ai-analysis
// Demonstrates using a local LLM (LM Studio) to analyze crawled page content.
// Requires LM Studio running at http://127.0.0.1:1234 with an OpenAI-compatible model loaded.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	// Connect to local LM Studio using the OpenAI-compatible provider.
	llm, err := scout.NewOpenAIProvider(
		scout.WithOpenAIBaseURL("http://127.0.0.1:1234/v1"),
		scout.WithOpenAIKey("lm-studio"),
		scout.WithOpenAIModel("zai-org/glm-4.7-flash"),
	)
	if err != nil {
		log.Fatal(err)
	}

	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = browser.Close() }()

	// Navigate to a single page for analysis.
	page, err := browser.NewPage("https://quotes.toscrape.com")
	if err != nil {
		log.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Page loaded, extracting content via bridge...")

	// Use the bridge extension for in-browser DOM-to-markdown conversion.
	bridge, err := page.Bridge()
	if err != nil {
		log.Fatal(err)
	}

	md, err := bridge.DOMMarkdown(scout.WithDOMMainOnly())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Sending %d chars to LLM...\n\n", len(md))

	// Call the LLM directly with truncated content.
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	systemPrompt := "You are a concise assistant. Respond in plain text, no markdown."
	userPrompt := "From the text below, list the authors and one quote from each. Keep it brief.\n\n" + md

	analysis, err := llm.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("AI Analysis:")
	fmt.Println(analysis)
}
