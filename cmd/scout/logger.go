package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/inovacc/scout/internal/flags"
	"github.com/spf13/cobra"
)

var loggerCmd = &cobra.Command{
	Use:   "logger",
	Short: "Configure scout command logging",
	Long: `Configure scout command logging.

Enable logging to a directory:
  scout logger --path /path/to/logs

To disable logging:
  scout logger --disable

To view all log files:
  scout logger --viewer

To check status:
  scout logger --status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logPath, _ := cmd.Flags().GetString("path")
		disable, _ := cmd.Flags().GetBool("disable")
		status, _ := cmd.Flags().GetBool("status")
		viewer, _ := cmd.Flags().GetBool("viewer")

		if err := flags.IgnoreCommand("logger"); err != nil {
			return err
		}

		if viewer {
			return viewLogs(cmd)
		}

		if status {
			return printStatus(cmd)
		}

		if disable {
			return flags.DisableFeature("logger")
		}

		if logPath == "" {
			return fmt.Errorf("--path is required (or use --disable to turn off logging)")
		}

		return flags.EnableFeature("logger", logPath)
	},
}

func printStatus(cmd *cobra.Command) error {
	logPath := flags.GetFeatureData("logger")

	if logPath == "" {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Logging: disabled (not configured)")
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Logging: Enabled\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Log path: %s\n", logPath)

	return nil
}

func viewLogs(cmd *cobra.Command) error {
	logDir := flags.GetFeatureData("logger")

	if logDir == "" {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Logging is not configured. Use --path to set log directory.")
		return nil
	}

	info, err := os.Stat(logDir)
	if os.IsNotExist(err) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Log directory does not exist: %s\n", logDir)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to access log directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("log path is not a directory: %s", logDir)
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	var logFiles []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.HasSuffix(entry.Name(), ".log") {
			logFiles = append(logFiles, entry.Name())
		}
	}

	if len(logFiles) == 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No log files found in: %s\n", logDir)
		return nil
	}

	// Sort by filename (KSUID prefix ensures chronological order)
	sort.Strings(logFiles)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Log files in %s (%d files):\n", logDir, len(logFiles))
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 60))

	for _, filename := range logFiles {
		filePath := filepath.Join(logDir, filename)

		cmdName := extractCommandName(filename)

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n[%s] %s\n", cmdName, filename)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 60))

		content, err := os.ReadFile(filePath)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Error reading file: %v\n", err)
			continue
		}

		if len(content) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (empty)")
			continue
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(content))
	}

	return nil
}

// extractCommandName extracts the command name from a log filename.
// Format: ksuid-command.log -> command
func extractCommandName(filename string) string {
	name := strings.TrimSuffix(filename, ".log")

	idx := strings.Index(name, "-")
	if idx == -1 || idx >= len(name)-1 {
		return "unknown"
	}

	return name[idx+1:]
}

func init() {
	rootCmd.AddCommand(loggerCmd)

	loggerCmd.Flags().StringP("path", "p", "", "Path to the log directory")
	loggerCmd.Flags().BoolP("disable", "d", false, "Disable logging")
	loggerCmd.Flags().BoolP("status", "s", false, "Show current logging status")
	loggerCmd.Flags().BoolP("viewer", "v", false, "View all log files sorted by time")
}
