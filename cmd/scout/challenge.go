package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

var challengeCmd = &cobra.Command{
	Use:   "challenge",
	Short: "Bot protection challenge detection and solving",
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

var challengeSolveCmd = &cobra.Command{
	Use:   "solve <url>",
	Short: "Navigate to a URL and attempt to solve bot protection challenges",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()

		var solverOpts []scout.SolverOption

		service, _ := cmd.Flags().GetString("service")
		apiKey, _ := cmd.Flags().GetString("api-key")
		if service != "" && apiKey != "" {
			switch service {
			case "2captcha":
				solverOpts = append(solverOpts, scout.WithSolverService(scout.NewTwoCaptchaService(apiKey)))
			case "capsolver":
				solverOpts = append(solverOpts, scout.WithSolverService(scout.NewCapSolverService(apiKey)))
			default:
				return fmt.Errorf("unknown solver service: %s (supported: 2captcha, capsolver)", service)
			}
		}

		solver := scout.NewChallengeSolver(b, solverOpts...)

		page, err := b.NewPage("")
		if err != nil {
			return err
		}

		if err := scout.NavigateWithBypass(page, args[0], solver); err != nil {
			return fmt.Errorf("scout: challenge solve: %w", err)
		}

		title, _ := page.Title()
		pageURL, _ := page.URL()
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Solved: %s - %s\n", title, pageURL)

		// Detect remaining challenges.
		remaining, err := page.DetectChallenges()
		if err == nil && len(remaining) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: %d challenge(s) still detected\n", len(remaining))
			for _, c := range remaining {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-15s  confidence=%.1f  %s\n", c.Type, c.Confidence, c.Details)
			}
		}
		return nil
	},
}

func init() {
	challengeSolveCmd.Flags().String("service", "", "Third-party solver service (2captcha, capsolver)")
	challengeSolveCmd.Flags().String("api-key", "", "API key for the solver service")

	challengeCmd.AddCommand(challengeDetectCmd)
	challengeCmd.AddCommand(challengeSolveCmd)
	rootCmd.AddCommand(challengeCmd)
}
