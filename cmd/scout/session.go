package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionCreateCmd, sessionDestroyCmd, sessionListCmd, sessionUseCmd)

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
		addr, _ := cmd.Flags().GetString("addr")
		client, conn, err := getClient(addr)
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
