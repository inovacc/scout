package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionListCmd,
		sessionDirListCmd, sessionDirPruneCmd, sessionDirCleanCmd, sessionDirRmCmd, sessionResetCmd)

	sessionListCmd.Flags().Bool("pending", false, "list directories the reaper could not remove (locked/stuck) instead of tracked sessions")

	sessionResetCmd.Flags().Bool("all", false, "reset all sessions")
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage browser sessions",
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tracked browser sessions",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if pending, _ := cmd.Flags().GetBool("pending"); pending {
			dirs := scout.PendingCleanup()
			if len(dirs) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No pending cleanup.")
				return nil
			}

			for _, d := range dirs {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), d)
			}

			return nil
		}

		ids, err := listTrackedSessions()
		if err != nil {
			return err
		}

		if len(ids) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No tracked sessions.")
			return nil
		}

		for _, id := range ids {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), id)
		}

		return nil
	},
}

// --- session directory management commands ---

var sessionDirListCmd = &cobra.Command{
	Use:   "list-local",
	Short: "List all local session directories with scout.pid metadata",
	RunE: func(cmd *cobra.Command, _ []string) error {
		sessions, err := scout.ListSessions()
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No local sessions.")
			return nil
		}

		for _, s := range sessions {
			reusable := ""
			if s.Info.Reusable {
				reusable = " [reusable]"
			}

			domain := ""
			if s.Info.Domain != "" {
				domain = fmt.Sprintf("  domain=%s", s.Info.Domain)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  headless=%v%s%s  last_used=%s\n",
				s.ID, s.Info.Browser, s.Info.Headless, reusable, domain,
				s.Info.LastUsed.Format("2006-01-02 15:04:05"))

			if s.Job != nil {
				pct := 0.0
				msg := ""

				if s.Job.Progress != nil {
					pct = s.Job.Progress.Percentage
					msg = s.Job.Progress.Message
				}

				line := fmt.Sprintf("  Job: %s [%s] %.0f%%", s.Job.Type, s.Job.Status, pct)
				if msg != "" {
					line += fmt.Sprintf(" — %s", msg)
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
			}
		}

		return nil
	},
}

var sessionDirPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove session dirs that have no scout.pid or are empty",
	RunE: func(cmd *cobra.Command, _ []string) error {
		sessDir := scout.SessionsDir()

		entries, err := os.ReadDir(sessDir)
		if err != nil {
			if os.IsNotExist(err) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No sessions directory.")
				return nil
			}

			return err
		}

		pruned := 0

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			pidPath := filepath.Join(sessDir, e.Name(), "scout.pid")
			if _, err := os.Stat(pidPath); os.IsNotExist(err) {
				if err := os.RemoveAll(filepath.Join(sessDir, e.Name())); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: remove %s: %v\n", e.Name(), err)
					continue
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pruned: %s (no scout.pid)\n", e.Name())
				pruned++
			}
		}

		if pruned == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune.")
		}

		return nil
	},
}

var sessionDirCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Delete all non-reusable session directories",
	RunE: func(cmd *cobra.Command, _ []string) error {
		sessions, err := scout.ListSessions()
		if err != nil {
			return err
		}

		cleaned := 0

		for _, s := range sessions {
			if s.Info.Reusable {
				continue
			}

			if err := os.RemoveAll(s.Dir); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: remove %s: %v\n", s.Dir, err)
				continue
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Cleaned: %s (%s)\n", s.ID, s.Dir)
			cleaned++
		}

		if cleaned == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No non-reusable sessions to clean.")
		}

		return nil
	},
}

var sessionDirRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove a specific session directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		dir := scout.SessionDir(id)

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("session %s not found", id)
		}

		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove session dir: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed session %s\n", id)

		return nil
	},
}

var sessionResetCmd = &cobra.Command{
	Use:   "reset [id]",
	Short: "Reset (delete) session data directories",
	Long:  "Removes session browser data and scout.pid. Kills browser process if still running.",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			n, err := scout.ResetAllSessions()
			if err != nil {
				return fmt.Errorf("scout: reset all sessions: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reset %d session(s)\n", n)

			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("session ID required (or use --all)")
		}

		if err := scout.ResetSession(args[0]); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reset session: %s\n", args[0])

		return nil
	},
}

// listTrackedSessions returns all session IDs from the engine's session
// directory. The legacy `active-sessions/` index has been removed — session
// presence on disk is now the single source of truth.
func listTrackedSessions() ([]string, error) {
	listings, err := scout.ListSessions()
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(listings))
	for _, l := range listings {
		ids = append(ids, l.ID)
	}

	return ids, nil
}
