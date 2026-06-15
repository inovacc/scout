package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/inovacc/scout/internal/flags"
	"github.com/inovacc/scout/internal/interaction"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(interactionsCmd)
	interactionsCmd.AddCommand(interactionsOnCmd, interactionsOffCmd, interactionsStatusCmd, interactionsListCmd)
	interactionsOnCmd.Flags().String("dir", "", "capture directory (default: <scouthome>/captures)")
	_ = flags.IgnoreCommand("interactions") // never capture the management command itself
}

var interactionsCmd = &cobra.Command{
	Use:   "interactions",
	Short: "Capture Scout interactions (CLI, MCP, browser actions) to <scouthome>/captures for analysis",
}

var interactionsOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable interaction capture",
	RunE: func(cmd *cobra.Command, _ []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if err := flags.EnableFeature("interactions", dir); err != nil {
			return fmt.Errorf("scout: interactions: %w", err)
		}

		d, _ := interaction.Dir()
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "interaction capture ON -> %s\n", d)

		return nil
	},
}

var interactionsOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable interaction capture",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := flags.DisableFeature("interactions"); err != nil {
			return fmt.Errorf("scout: interactions: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "interaction capture OFF")

		return nil
	},
}

var interactionsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show interaction-capture status",
	RunE: func(cmd *cobra.Command, _ []string) error {
		d, _ := interaction.Dir()
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "enabled: %v\ndir: %s\n", interaction.Enabled(), d)

		return nil
	},
}

var interactionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List capture files",
	RunE: func(cmd *cobra.Command, _ []string) error {
		d, err := interaction.Dir()
		if err != nil {
			return err
		}

		entries, err := os.ReadDir(d)
		if err != nil {
			if os.IsNotExist(err) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no captures")
				return nil
			}

			return err
		}

		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
				names = append(names, e.Name())
			}
		}

		sort.Strings(names)

		for _, n := range names {
			size := int64(0)
			if info, statErr := os.Stat(filepath.Join(d, n)); statErr == nil {
				size = info.Size()
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-40s %8d bytes\n", n, size)
		}

		if len(names) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no captures")
		}

		return nil
	},
}
