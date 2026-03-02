package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionCreateCmd, sessionDestroyCmd, sessionListCmd, sessionUseCmd,
		sessionDirListCmd, sessionDirPruneCmd, sessionDirCleanCmd, sessionDirRmCmd)

	sessionCreateCmd.Flags().Bool("headless", true, "run browser in headless mode")
	sessionCreateCmd.Flags().Bool("stealth", false, "enable anti-bot-detection stealth mode")
	sessionCreateCmd.Flags().String("proxy", "", "proxy URL (e.g. socks5://host:port)")
	sessionCreateCmd.Flags().String("user-agent", "", "custom user agent string")
	sessionCreateCmd.Flags().String("url", "", "initial URL to navigate to")
	sessionCreateCmd.Flags().Bool("record", false, "enable HAR recording")
	sessionCreateCmd.Flags().Bool("capture-body", false, "capture response bodies in HAR")
	sessionCreateCmd.Flags().Bool("maximized", false, "start browser window maximized")
	sessionCreateCmd.Flags().Bool("devtools", false, "open Chrome DevTools automatically")
	sessionCreateCmd.Flags().Bool("no-sandbox", false, "disable browser sandbox (containers/WSL)")
	sessionCreateCmd.Flags().String("profile", "", "path to .scoutprofile file to apply at session creation")
	sessionCreateCmd.Flags().Bool("decrypt", false, "decrypt the profile file (requires --passphrase)")
	sessionCreateCmd.Flags().String("passphrase", "", "passphrase for encrypted profile decryption")

	sessionDestroyCmd.Flags().Bool("all", false, "destroy all sessions")
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage browser sessions",
}

var sessionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new browser session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		insecureMode, _ := cmd.Flags().GetBool("insecure")

		var (
			client pb.ScoutServiceClient
			conn   *grpc.ClientConn
			err    error
		)
		if insecureMode {
			if err := ensureDaemon(addr); err != nil {
				return err
			}

			client, conn, err = getClient(addr)
		} else {
			client, conn, err = getClientTLS(addr)
		}

		if err != nil {
			return err
		}

		defer func() { _ = conn.Close() }()

		headless, _ := cmd.Flags().GetBool("headless")
		stealth, _ := cmd.Flags().GetBool("stealth")
		proxy, _ := cmd.Flags().GetString("proxy")
		userAgent, _ := cmd.Flags().GetString("user-agent")
		url, _ := cmd.Flags().GetString("url")
		record, _ := cmd.Flags().GetBool("record")
		captureBody, _ := cmd.Flags().GetBool("capture-body")
		maximized, _ := cmd.Flags().GetBool("maximized")
		devtools, _ := cmd.Flags().GetBool("devtools")
		noSandbox, _ := cmd.Flags().GetBool("no-sandbox")

		// Load profile if specified, applying overrides to session creation fields.
		profilePath, _ := cmd.Flags().GetString("profile")
		if profilePath != "" {
			decrypt, _ := cmd.Flags().GetBool("decrypt")

			var prof *scout.UserProfile

			if decrypt {
				passphrase, _ := cmd.Flags().GetString("passphrase")
				if passphrase == "" {
					passphrase, err = readPassphrase(cmd.ErrOrStderr(), "Enter passphrase: ")
					if err != nil {
						return err
					}
				}

				prof, err = scout.LoadProfileEncrypted(profilePath, passphrase)
			} else {
				prof, err = scout.LoadProfile(profilePath)
			}

			if err != nil {
				return fmt.Errorf("scout: session create: load profile: %w", err)
			}

			// Apply profile fields as defaults (explicit flags still win).
			if !cmd.Flags().Changed("user-agent") && prof.Identity.UserAgent != "" {
				userAgent = prof.Identity.UserAgent
			}

			if !cmd.Flags().Changed("proxy") && prof.Proxy != "" {
				proxy = prof.Proxy
			}

			if !cmd.Flags().Changed("stealth") && prof.Browser.Type == "brave" {
				stealth = true
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Applying profile: %s\n", prof.Name)
		}

		resp, err := client.CreateSession(context.Background(), &pb.CreateSessionRequest{
			Headless:    headless,
			Stealth:     stealth,
			Proxy:       proxy,
			UserAgent:   userAgent,
			InitialUrl:  url,
			Record:      record,
			CaptureBody: captureBody,
			Maximized:   maximized,
			Devtools:    devtools,
			NoSandbox:   noSandbox,
		})
		if err != nil {
			return fmt.Errorf("scout: create session: %w", err)
		}

		if err := saveCurrentSession(resp.GetSessionId()); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not save session: %v\n", err)
		}

		if err := trackSession(resp.GetSessionId()); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not track session: %v\n", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Session created: %s\n", resp.GetSessionId())
		if resp.GetTitle() != "" || resp.GetUrl() != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Page: %s - %s\n", resp.GetTitle(), resp.GetUrl())
		}

		return nil
	},
}

var sessionDestroyCmd = &cobra.Command{
	Use:   "destroy [id]",
	Short: "Destroy a browser session",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, conn, err := resolveClient(cmd)
		if err != nil {
			return err
		}

		defer func() { _ = conn.Close() }()

		all, _ := cmd.Flags().GetBool("all")
		if all {
			ids, _ := listTrackedSessions()
			for _, id := range ids {
				_, err := client.DestroySession(context.Background(), &pb.SessionRequest{SessionId: id})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: destroy %s: %v\n", id, err)
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Destroyed: %s\n", id)
				}

				untrackSession(id)
			}

			return nil
		}

		sessionFlag, _ := cmd.Flags().GetString("session")

		var id string
		if len(args) > 0 {
			id = args[0]
		} else {
			id, err = resolveSession(sessionFlag)
			if err != nil {
				return err
			}
		}

		_, err = client.DestroySession(context.Background(), &pb.SessionRequest{SessionId: id})
		if err != nil {
			return fmt.Errorf("scout: destroy session: %w", err)
		}

		untrackSession(id)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Destroyed: %s\n", id)

		return nil
	},
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tracked browser sessions",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ids, err := listTrackedSessions()
		if err != nil {
			return err
		}

		if len(ids) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No tracked sessions.")
			return nil
		}

		currentID, _ := resolveSession("")

		for _, id := range ids {
			marker := "  "
			if id == currentID {
				marker = "* "
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", marker, id)
		}

		return nil
	},
}

var sessionUseCmd = &cobra.Command{
	Use:   "use <id>",
	Short: "Set the active session for subsequent commands",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := saveCurrentSession(args[0]); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Active session: %s\n", args[0])

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

// trackSession records a session ID in ~/.scout/sessions/.
func trackSession(id string) error {
	dir, err := scoutDir()
	if err != nil {
		return err
	}

	sessDir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(sessDir, id), nil, 0o644)
}

// untrackSession removes a tracked session.
func untrackSession(id string) {
	dir, _ := scoutDir()
	if dir != "" {
		_ = os.Remove(filepath.Join(dir, "sessions", id))
	}
}

// listTrackedSessions returns all tracked session IDs.
func listTrackedSessions() ([]string, error) {
	dir, err := scoutDir()
	if err != nil {
		return nil, err
	}

	sessDir := filepath.Join(dir, "sessions")

	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var ids []string

	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			ids = append(ids, e.Name())
		}
	}

	return ids, nil
}
