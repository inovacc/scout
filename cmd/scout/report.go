package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/inovacc/scout/pkg/scout/tools"
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
	Short: "List all saved reports (also: MCP tool `mcp__scout__report_list`)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		out, err := tools.ReportList(context.Background(), tools.ReportListInput{})
		if err != nil {
			return err
		}
		if out.Total == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No reports found.")
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tTYPE\tURL\tISSUES\tCREATED")
		for _, r := range out.Reports {
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
	Short: "Show a saved report (also: MCP tool `mcp__scout__report_show`)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := tools.ReportShow(context.Background(), tools.ReportShowInput{ID: args[0], Raw: true})
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(cmd.OutOrStdout(), out.Raw)
		return nil
	},
}

var reportDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a saved report (also: MCP tool `mcp__scout__report_delete`)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := tools.ReportDelete(context.Background(), tools.ReportDeleteInput{ID: args[0]})
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted report: %s\n", out.ID)
		return nil
	},
}
