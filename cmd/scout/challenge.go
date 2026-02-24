package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

var challengeCmd = &cobra.Command{
	Use:   "challenge",
	Short: "Bot protection challenge detection",
}

var challengeDetectCmd = &cobra.Command{
	Use:   "detect <url>",
	Short: "Detect bot protection challenges on a page",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()

		page, err := b.NewPage(args[0])
		if err != nil {
			return err
		}
		if err := page.WaitLoad(); err != nil {
			return err
		}

		challenges, err := page.DetectChallenges()
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			data, _ := json.MarshalIndent(challenges, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		} else {
			if len(challenges) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No bot protection challenges detected.")
				return nil
			}
			for _, c := range challenges {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-15s  confidence=%.1f  %s\n", c.Type, c.Confidence, c.Details)
			}
		}
		return nil
	},
}

func init() {
	challengeCmd.AddCommand(challengeDetectCmd)
	rootCmd.AddCommand(challengeCmd)
}
