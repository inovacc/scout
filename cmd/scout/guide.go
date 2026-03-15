package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/guide"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(guideCmd)

	guideCmd.Flags().String("title", "", "guide title (defaults to page title)")
	guideCmd.Flags().StringP("output", "o", "guide.md", "output markdown file")
}

var guideCmd = &cobra.Command{
	Use:   "guide <url>",
	Short: "Record a step-by-step guide with screenshots",
	Long:  "Navigate to a URL and interactively record steps. Each step captures a screenshot and optional annotation. The result is a markdown file with embedded screenshots.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := args[0]

		opts := baseOpts(cmd)
		opts = append(opts, scout.WithHeadless(false)) // guides need a visible browser

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: guide: %w", err)
		}

		defer func() { _ = b.Close() }()

		page, err := b.NewPage(targetURL)
		if err != nil {
			return fmt.Errorf("scout: guide: %w", err)
		}

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: guide: wait load: %w", err)
		}

		title, _ := cmd.Flags().GetString("title")
		if title == "" {
			t, err := page.Title()
			if err == nil && t != "" {
				title = t
			} else {
				title = "Untitled Guide"
			}
		}

		rec := guide.NewRecorder()

		if err := rec.Start(title, targetURL); err != nil {
			return fmt.Errorf("scout: guide: %w", err)
		}

		// Take initial screenshot as step 1.
		screenshot, err := page.Screenshot()
		if err != nil {
			return fmt.Errorf("scout: guide: screenshot: %w", err)
		}

		pageURL, _ := page.URL()
		pageTitle, _ := page.Title()

		if err := rec.AddStep(pageURL, pageTitle, "Initial page", screenshot); err != nil {
			return fmt.Errorf("scout: guide: add step: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Guide started. Press Enter to capture a step, type 'q' then Enter to finish.")

		scanner := bufio.NewScanner(os.Stdin)

		for {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), "> ")

			if !scanner.Scan() {
				break
			}

			input := strings.TrimSpace(scanner.Text())
			if input == "q" || input == "quit" || input == "exit" {
				break
			}

			// Use input as annotation if provided, otherwise prompt.
			annotation := input
			if annotation == "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), "Annotation (optional): ")

				if scanner.Scan() {
					annotation = strings.TrimSpace(scanner.Text())
				}
			}

			ss, err := page.Screenshot()
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "screenshot error: %v\n", err)
				continue
			}

			pURL, _ := page.URL()
			pTitle, _ := page.Title()

			if err := rec.AddStep(pURL, pTitle, annotation, ss); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "add step error: %v\n", err)
				continue
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Step recorded: %s (%s)\n", pTitle, pURL)
		}

		g, err := rec.Finish()
		if err != nil {
			return fmt.Errorf("scout: guide: %w", err)
		}

		md, err := guide.RenderMarkdown(g)
		if err != nil {
			return fmt.Errorf("scout: guide: render: %w", err)
		}

		outFile, _ := cmd.Flags().GetString("output")

		if err := os.WriteFile(outFile, md, 0o644); err != nil {
			return fmt.Errorf("scout: guide: write: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Guide saved: %s (%d steps)\n", outFile, len(g.Steps))

		return nil
	},
}
