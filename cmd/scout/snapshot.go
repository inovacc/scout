package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(snapshotCmd)

	snapshotCmd.Flags().String("format", "yaml", "output format: yaml, json")
	snapshotCmd.Flags().Bool("iframes", false, "include iframe content in snapshot")
	snapshotCmd.Flags().Bool("refs", true, "include ref markers (default true)")
	snapshotCmd.Flags().Int("max-depth", 0, "max DOM traversal depth (0 = unlimited)")
	snapshotCmd.Flags().Bool("interactable", false, "only include interactable elements")
}

var snapshotCmd = &cobra.Command{
	Use:   "snapshot <url>",
	Short: "Capture accessibility tree snapshot of a web page",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(url)
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: wait load: %w", err)
		}

		var opts []scout.SnapshotOption

		if iframes, _ := cmd.Flags().GetBool("iframes"); iframes {
			opts = append(opts, scout.WithSnapshotIframes())
		}

		if maxDepth, _ := cmd.Flags().GetInt("max-depth"); maxDepth > 0 {
			opts = append(opts, scout.WithSnapshotMaxDepth(maxDepth))
		}

		if interactable, _ := cmd.Flags().GetBool("interactable"); interactable {
			opts = append(opts, scout.WithSnapshotInteractableOnly())
		}

		snap, err := page.SnapshotWithOptions(opts...)
		if err != nil {
			return fmt.Errorf("scout: snapshot: %w", err)
		}

		format, _ := cmd.Flags().GetString("format")
		switch format {
		case "json":
			data, err := json.Marshal(map[string]string{"snapshot": snap})
			if err != nil {
				return fmt.Errorf("scout: json encode: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		default:
			outFile, _ := cmd.Flags().GetString("output")
			if outFile != "" {
				dest, err := writeOutput(cmd, []byte(snap), "snapshot.yaml")
				if err != nil {
					return err
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Written to %s\n", dest)

				return nil
			}

			_, _ = fmt.Fprint(cmd.OutOrStdout(), snap)
		}

		return nil
	},
}
