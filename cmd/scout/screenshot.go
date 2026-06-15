package main

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/spf13/cobra"
)

// Note: the canonical single-shot `scout screenshot <url>` lives in
// mcp_screenshot.go (rootScreenshotCmd). This file carries only `pdf`.
func init() {
	rootCmd.AddCommand(pdfCmd)
}

var pdfCmd = &cobra.Command{
	Use:   "pdf <url>",
	Short: "Generate a PDF of a page",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		b, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: pdf: %w", err)
		}

		defer func() { _ = b.Close() }()

		page, err := b.NewPage(url)
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: wait load: %w", err)
		}

		out, err := tools.PDF(context.Background(), page, tools.PDFInput{})
		if err != nil {
			return fmt.Errorf("scout: pdf: %w", err)
		}

		defaultName := fmt.Sprintf("page_%d.pdf", time.Now().Unix())

		filename, err := writeOutput(cmd, out.Data, defaultName)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved to %s (%d bytes)\n", filename, len(out.Data))

		return nil
	},
}
