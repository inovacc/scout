package main

import (
	"context"
	"fmt"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(windowCmd)
	windowCmd.AddCommand(windowGetCmd, windowMinCmd, windowMaxCmd, windowFullCmd, windowRestoreCmd)
}

var windowCmd = &cobra.Command{
	Use:   "window",
	Short: "Control browser window state",
}

var windowGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current window bounds",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, conn, err := resolveClient(cmd)
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
			Script:    `JSON.stringify({width: window.outerWidth, height: window.outerHeight, x: window.screenX, y: window.screenY})`,
		})
		if err != nil {
			return fmt.Errorf("scout: window get: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetResult())
		return nil
	},
}

func windowEvalCmd(name, js string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("%s the browser window", name),
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

			// Use CDP via eval to change window state — the gRPC server
			// doesn't expose window state RPCs, so we use JS workarounds
			_, err = client.Eval(context.Background(), &pb.EvalRequest{
				SessionId: sessionID,
				Script:    js,
			})
			if err != nil {
				return fmt.Errorf("scout: window %s: %w", name, err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "window %sd\n", name)

			return nil
		},
	}
}

var (
	windowMinCmd     = windowEvalCmd("minimize", `window.minimize ? window.minimize() : null`)
	windowMaxCmd     = windowEvalCmd("maximize", `window.moveTo(0,0); window.resizeTo(screen.availWidth, screen.availHeight)`)
	windowFullCmd    = windowEvalCmd("fullscreen", `document.documentElement.requestFullscreen()`)
	windowRestoreCmd = windowEvalCmd("restore", `document.exitFullscreen && document.exitFullscreen()`)
)
