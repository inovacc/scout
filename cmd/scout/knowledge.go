package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(knowledgeCmd)

	knowledgeCmd.Flags().Int("depth", 3, "BFS crawl depth")
	knowledgeCmd.Flags().Int("max-pages", 100, "Maximum pages to visit")
	knowledgeCmd.Flags().Int("concurrency", 1, "Concurrent page processing")
	knowledgeCmd.Flags().Duration("timeout", 30*time.Second, "Per-page timeout")
	knowledgeCmd.Flags().String("output", "", "Output directory (default: knowledge-{domain}/)")
	knowledgeCmd.Flags().Bool("json", false, "Output single JSON blob to stdout")
}

var knowledgeCmd = &cobra.Command{
	Use:   "knowledge <url>",
	Short: "Crawl a site and collect all possible intelligence",
	Long: `Crawl a site and collect all possible intelligence per page:
markdown, HTML, links, meta, cookies, screenshots, accessibility snapshots,
HAR traffic, tech stack, console logs, Swagger/API docs, and PDFs.

Output is both a structured directory and optionally a single JSON blob.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := args[0]
		if !strings.HasPrefix(targetURL, "http") {
			targetURL = "https://" + targetURL
		}

		depth, _ := cmd.Flags().GetInt("depth")
		maxPages, _ := cmd.Flags().GetInt("max-pages")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		outputDir, _ := cmd.Flags().GetString("output")
		jsonOut, _ := cmd.Flags().GetBool("json")

		if outputDir == "" && !jsonOut {
			outputDir = "knowledge-" + domainFromURL(targetURL)
		}

		opts := baseOpts(cmd)

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("create browser: %w", err)
		}

		defer func() { _ = b.Close() }()

		var kOpts []scout.KnowledgeOption

		kOpts = append(kOpts, scout.WithKnowledgeDepth(depth))
		kOpts = append(kOpts, scout.WithKnowledgeMaxPages(maxPages))
		kOpts = append(kOpts, scout.WithKnowledgeConcurrency(concurrency))

		kOpts = append(kOpts, scout.WithKnowledgeTimeout(timeout))
		if outputDir != "" {
			kOpts = append(kOpts, scout.WithKnowledgeOutput(outputDir))
		}

		_, _ = fmt.Fprintf(os.Stderr, "Crawling %s (depth=%d, max-pages=%d)...\n", targetURL, depth, maxPages)

		result, err := b.Knowledge(targetURL, kOpts...)
		if err != nil {
			return err
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")

			return enc.Encode(result)
		}

		_, _ = fmt.Fprintf(os.Stderr, "\nKnowledge collection complete:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  Pages:  %d (%d ok, %d failed)\n",
			result.Summary.PagesTotal, result.Summary.PagesSuccess, result.Summary.PagesFailed)
		_, _ = fmt.Fprintf(os.Stderr, "  Links:  %d unique\n", result.Summary.UniqueLinks)

		_, _ = fmt.Fprintf(os.Stderr, "  Time:   %s\n", result.Duration)
		if outputDir != "" {
			_, _ = fmt.Fprintf(os.Stderr, "  Output: %s/\n", outputDir)
		}

		if result.TechStack != nil && len(result.TechStack.Frameworks) > 0 {
			names := make([]string, len(result.TechStack.Frameworks))
			for i, f := range result.TechStack.Frameworks {
				names[i] = f.Name
			}

			_, _ = fmt.Fprintf(os.Stderr, "  Stack:  %s\n", strings.Join(names, ", "))
		}

		return nil
	},
}

func domainFromURL(rawURL string) string {
	s := rawURL
	s = strings.TrimPrefix(s, "https://")

	s = strings.TrimPrefix(s, "http://")
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}

	return s
}
