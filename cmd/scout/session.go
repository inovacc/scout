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
		sessionTrackListCmd, sessionTrackPruneCmd, sessionTrackCleanCmd, sessionTrackRmCmd)

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

// --- track.json session management commands ---

var sessionTrackListCmd = &cobra.Command{
	Use:   "track-list",
	Short: "List all sessions tracked in track.json (local data dirs)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		tracker, err := scout.LoadTracker()
		if err != nil {
			return err
		}

		if len(tracker.Sessions) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No tracked sessions in track.json.")
			return nil
		}

		for _, s := range tracker.Sessions {
			reusable := ""
			if s.Reusable {
				reusable = " [reusable]"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  headless=%v%s  last_used=%s\n",
				s.ID, s.Browser, s.DataDir, s.Headless, reusable,
				s.LastUsed.Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}

var sessionTrackPruneCmd = &cobra.Command{
	Use:   "track-prune",
	Short: "Remove stale entries from track.json (data dir no longer exists)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		tracker, err := scout.LoadTracker()
		if err != nil {
			return err
		}

		pruned, err := tracker.Prune()
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d stale session(s).\n", pruned)
		return nil
	},
}

var sessionTrackCleanCmd = &cobra.Command{
	Use:   "track-clean",
	Short: "Delete all non-reusable session data directories",
	RunE: func(cmd *cobra.Command, _ []string) error {
		tracker, err := scout.LoadTracker()
		if err != nil {
			return err
		}

		var toRemove []scout.SessionEntry
		for _, s := range tracker.Sessions {
			if !s.Reusable {
				toRemove = append(toRemove, s)
			}
		}

		for _, s := range toRemove {
			if err := os.RemoveAll(s.DataDir); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: remove %s: %v\n", s.DataDir, err)
			}
			_ = tracker.Remove(s.ID)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Cleaned: %s (%s)\n", s.ID, s.DataDir)
		}

		if len(toRemove) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No non-reusable sessions to clean.")
		}

		return nil
	},
}

var sessionTrackRmCmd = &cobra.Command{
	Use:   "track-rm <id>",
	Short: "Remove a specific session and its data directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tracker, err := scout.LoadTracker()
		if err != nil {
			return err
		}

		id := args[0]

		// Find the entry to get the data dir.
		var dataDir string
		for _, s := range tracker.Sessions {
			if s.ID == id {
				dataDir = s.DataDir
				break
			}
		}

		if dataDir == "" {
			return fmt.Errorf("session %s not found in track.json", id)
		}

		if err := os.RemoveAll(dataDir); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: remove data dir: %v\n", err)
		}

		if err := tracker.Remove(id); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed session %s and data dir %s\n", id, dataDir)
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
