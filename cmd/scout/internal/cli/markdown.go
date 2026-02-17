package cli

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(markdownCmd)

	markdownCmd.Flags().String("url", "", "URL to convert to markdown (required)")
	markdownCmd.Flags().Bool("main-only", false, "extract only the main content")
	markdownCmd.Flags().Bool("no-images", false, "exclude images from output")
	markdownCmd.Flags().Bool("no-links", false, "render links as plain text")
}

var markdownCmd = &cobra.Command{
	Use:   "markdown",
	Short: "Convert a web page to Markdown",
	RunE: func(cmd *cobra.Command, _ []string) error {
		urlFlag, _ := cmd.Flags().GetString("url")
		if urlFlag == "" {
			return fmt.Errorf("scout: --url is required")
		}

		mainOnly, _ := cmd.Flags().GetBool("main-only")
		noImages, _ := cmd.Flags().GetBool("no-images")
		noLinks, _ := cmd.Flags().GetBool("no-links")

		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(urlFlag)
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}
		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: wait load: %w", err)
		}

		var opts []scout.MarkdownOption
		if mainOnly {
			opts = append(opts, scout.WithMainContentOnly())
		}
		if noImages {
			opts = append(opts, scout.WithIncludeImages(false))
		}
		if noLinks {
			opts = append(opts, scout.WithIncludeLinks(false))
		}

		md, err := page.Markdown(opts...)
		if err != nil {
			return fmt.Errorf("scout: markdown: %w", err)
		}

		outFile, _ := cmd.Flags().GetString("output")
		if outFile != "" {
			dest, err := writeOutput(cmd, []byte(md), "page.md")
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Written to %s\n", dest)
			return nil
		}

		_, _ = fmt.Fprint(cmd.OutOrStdout(), md)
		return nil
	},
}
