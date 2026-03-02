package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(recordCmd)

	recordCmd.Flags().Int("quality", 80, "JPEG quality (1-100)")
	recordCmd.Flags().Int("width", 0, "maximum frame width (0 = no limit)")
	recordCmd.Flags().Int("height", 0, "maximum frame height (0 = no limit)")
	recordCmd.Flags().String("frames", "", "export individual JPEG frames to directory")
}

var recordCmd = &cobra.Command{
	Use:   "record <url>",
	Short: "Record browser screen as animated GIF",
	Long:  "Navigate to a URL and record the screen via CDP screencast. Press Ctrl+C to stop. Exports as GIF (--output) or individual frames (--frames).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		quality, _ := cmd.Flags().GetInt("quality")
		width, _ := cmd.Flags().GetInt("width")
		height, _ := cmd.Flags().GetInt("height")
		framesDir, _ := cmd.Flags().GetString("frames")

		opts := baseOpts(cmd)

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: record: %w", err)
		}

		defer func() { _ = b.Close() }()

		page, err := b.NewPage(url)
		if err != nil {
			return fmt.Errorf("scout: record: %w", err)
		}

		_ = page.WaitLoad()

		recOpts := []scout.ScreenRecordOption{
			scout.WithRecordQuality(quality),
		}
		if width > 0 || height > 0 {
			recOpts = append(recOpts, scout.WithRecordSize(width, height))
		}

		rec := scout.NewScreenRecorder(page, recOpts...)
		if err := rec.Start(); err != nil {
			return fmt.Errorf("scout: record: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "recording... press Ctrl+C to stop")

		// Wait for interrupt.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		signal.Stop(sigCh)

		if err := rec.Stop(); err != nil {
			return fmt.Errorf("scout: record: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "captured %d frames (duration: %v)\n", rec.FrameCount(), rec.Duration())

		if rec.FrameCount() == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no frames captured")
			return nil
		}

		// Export individual frames if requested.
		if framesDir != "" {
			if err := rec.ExportFrames(framesDir); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "frames exported to %s\n", framesDir)
		}

		// Export GIF.
		defaultName := fmt.Sprintf("recording_%d.gif", time.Now().Unix())

		outFile, _ := cmd.Flags().GetString("output")
		if outFile == "" {
			outFile = defaultName
		}

		if outFile == "-" {
			return rec.ExportGIF(cmd.OutOrStdout())
		}

		f, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("scout: record: create file: %w", err)
		}

		defer func() { _ = f.Close() }()

		if err := rec.ExportGIF(f); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "GIF exported: %s\n", outFile)

		return nil
	},
}
