package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(researchCmd)

	researchCmd.Flags().Int("sources", 5, "max number of sources to fetch")
	researchCmd.Flags().Int("depth", 1, "research depth (1 = single pass, >1 = follow-up iterations)")
	researchCmd.Flags().String("engine", "google", "search engine: google, bing, duckduckgo")
	researchCmd.Flags().String("provider", "ollama", "LLM provider: ollama, openai, anthropic, openrouter, deepseek, gemini")
	researchCmd.Flags().String("model", "", "model name (default: provider-specific)")
	researchCmd.Flags().String("api-key", "", "API key for remote providers (or use env)")
	researchCmd.Flags().String("api-base", "", "custom API base URL")
	researchCmd.Flags().String("ollama-host", "", "Ollama server URL")
	researchCmd.Flags().Duration("timeout", 2*time.Minute, "overall research timeout")
	researchCmd.Flags().Int("concurrency", 3, "fetch concurrency")
	researchCmd.Flags().Bool("main-only", true, "extract main content only")
	researchCmd.Flags().Bool("deep", false, "enable deep research (multi-iteration with follow-ups)")
}

var researchCmd = &cobra.Command{
	Use:   "research <query>",
	Short: "Research a topic using web search, fetching, and LLM synthesis",
	Long: `Perform multi-source research by searching the web, fetching top results,
and synthesizing findings with an LLM. Returns a summary with source attribution
and follow-up questions.

Use --deep for multi-iteration research that follows up on generated questions.

Supported LLM providers: ollama, openai, anthropic, openrouter, deepseek, gemini.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		sources, _ := cmd.Flags().GetInt("sources")
		depth, _ := cmd.Flags().GetInt("depth")
		engine, _ := cmd.Flags().GetString("engine")
		providerName, _ := cmd.Flags().GetString("provider")
		modelFlag, _ := cmd.Flags().GetString("model")
		apiKey, _ := cmd.Flags().GetString("api-key")
		apiBase, _ := cmd.Flags().GetString("api-base")
		ollamaHost, _ := cmd.Flags().GetString("ollama-host")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		mainOnly, _ := cmd.Flags().GetBool("main-only")
		deep, _ := cmd.Flags().GetBool("deep")
		format, _ := cmd.Flags().GetString("format")

		provider, err := createProviderFull(providerName, modelFlag, apiKey, apiBase, ollamaHost)
		if err != nil {
			return fmt.Errorf("scout: create provider: %w", err)
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		var searchEngine scout.SearchEngine

		switch engine {
		case "bing":
			searchEngine = scout.Bing
		case "duckduckgo", "ddg":
			searchEngine = scout.DuckDuckGo
		default:
			searchEngine = scout.Google
		}

		agent := scout.NewResearchAgent(browser, provider,
			scout.WithResearchMaxSources(sources),
			scout.WithResearchDepth(depth),
			scout.WithResearchEngine(searchEngine),
			scout.WithResearchTimeout(timeout),
			scout.WithResearchConcurrency(concurrency),
			scout.WithResearchMainContent(mainOnly),
		)

		var result *scout.ResearchResult
		if deep || depth > 1 {
			result, err = agent.DeepResearch(cmd.Context(), query)
		} else {
			result, err = agent.Research(cmd.Context(), query)
		}

		if err != nil {
			return err
		}

		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(result)
		}

		// Text output
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Research: %s\n", result.Query)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Duration: %s | Sources: %d | Depth: %d\n\n",
			result.Duration.Round(time.Millisecond), len(result.Sources), result.Depth)

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "## Summary\n\n%s\n\n", result.Summary)

		if len(result.Sources) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "## Sources\n\n")
			for i, src := range result.Sources {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d. [%.0f%%] %s\n   %s\n",
					i+1, src.Relevance*100, src.Title, src.URL)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}

		if len(result.FollowUpQuestions) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "## Follow-up Questions\n\n")
			for i, q := range result.FollowUpQuestions {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n", i+1, q)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}

		outFile, _ := cmd.Flags().GetString("output")
		if outFile != "" {
			data, _ := json.MarshalIndent(result, "", "  ")

			dest, writeErr := writeOutput(cmd, data, "research.json")
			if writeErr != nil {
				return writeErr
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Written to %s\n", dest)
		}

		return nil
	},
}
