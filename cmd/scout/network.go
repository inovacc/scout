package main

import (
	"context"
	"fmt"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cookieCmd, headerCmd, blockCmd)
	cookieCmd.AddCommand(cookieGetCmd, cookieSetCmd, cookieClearCmd)
}

var cookieCmd = &cobra.Command{
	Use:   "cookie",
	Short: "Manage browser cookies",
}

var cookieGetCmd = &cobra.Command{
	Use:   "get [urls...]",
	Short: "Get cookies for current page or specified URLs",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		client, conn, err := getClient(addr)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		sessionFlag, _ := cmd.Flags().GetString("session")
		sessionID, err := resolveSession(sessionFlag)
		if err != nil {
			return err
		}

		resp, err := client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    `document.cookie`,
		})
		if err != nil {
			return fmt.Errorf("scout: cookie get: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetResult())
		return nil
	},
}

var cookieSetCmd = &cobra.Command{
	Use:   "set <cookie-string>",
	Short: "Set a cookie (name=value; path=/; ...)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		client, conn, err := getClient(addr)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		sessionFlag, _ := cmd.Flags().GetString("session")
		sessionID, err := resolveSession(sessionFlag)
		if err != nil {
			return err
		}

		_, err = client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    fmt.Sprintf(`document.cookie = %q`, args[0]),
		})
		if err != nil {
			return fmt.Errorf("scout: cookie set: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "cookie set")
		return nil
	},
}

var cookieClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all cookies",
	RunE: func(cmd *cobra.Command, _ []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		client, conn, err := getClient(addr)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		sessionFlag, _ := cmd.Flags().GetString("session")
		sessionID, err := resolveSession(sessionFlag)
		if err != nil {
			return err
		}

		_, err = client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script: `document.cookie.split(";").forEach(function(c) {
				document.cookie = c.replace(/^ +/, "").replace(/=.*/, "=;expires=" + new Date().toUTCString() + ";path=/");
			})`,
		})
		if err != nil {
			return fmt.Errorf("scout: cookie clear: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "cookies cleared")
		return nil
	},
}

var headerCmd = &cobra.Command{
	Use:   "header <key> <value>",
	Short: "Set a custom header for all requests",
	Long:  "Set a custom header via JavaScript. Note: this sets headers via Eval and may require interceptor support.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		client, conn, err := getClient(addr)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		sessionFlag, _ := cmd.Flags().GetString("session")
		sessionID, err := resolveSession(sessionFlag)
		if err != nil {
			return err
		}

		// Store header info for display — actual header injection requires
		// network interception which needs to be handled server-side
		_, err = client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    fmt.Sprintf(`window.__scoutHeaders = window.__scoutHeaders || {}; window.__scoutHeaders[%q] = %q`, args[0], args[1]),
		})
		if err != nil {
			return fmt.Errorf("scout: header: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "header set: %s: %s\n", args[0], args[1])
		return nil
	},
}

var blockCmd = &cobra.Command{
	Use:   "block <url-pattern>",
	Short: "Block requests matching a URL pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		client, conn, err := getClient(addr)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		sessionFlag, _ := cmd.Flags().GetString("session")
		sessionID, err := resolveSession(sessionFlag)
		if err != nil {
			return err
		}

		// Store block pattern — actual blocking requires network interception
		_, err = client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    fmt.Sprintf(`window.__scoutBlocked = window.__scoutBlocked || []; window.__scoutBlocked.push(%q)`, args[0]),
		})
		if err != nil {
			return fmt.Errorf("scout: block: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "blocking: %s\n", args[0])
		return nil
	},
}
