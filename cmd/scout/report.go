package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.AddCommand(reportListCmd, reportShowCmd, reportDeleteCmd)
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Manage saved health check and issue reports",
}

var reportListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved reports",
	RunE: func(cmd *cobra.Command, _ []string) error {
		reports, err := scout.ListReports()
		if err != nil {
			return err
		}

		if len(reports) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No reports found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tTYPE\tURL\tISSUES\tCREATED")

		for _, r := range reports {
			issues := 0
			if r.Health != nil {
				issues = len(r.Health.Issues)
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
				r.ID, r.Type, truncate(r.URL, 40), issues,
				r.CreatedAt.Format("2006-01-02 15:04:05"))
		}

		return w.Flush()
	},
}

var reportShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a saved report",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := scout.ReadReport(args[0])
		if err != nil {
			return err
		}

		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")

		return enc.Encode(r)
	},
}

var reportDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a saved report",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := scout.DeleteReport(args[0]); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted report: %s\n", args[0])

		return nil
	},
}
