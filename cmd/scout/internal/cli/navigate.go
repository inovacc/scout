package cli

import (
	"context"
	"fmt"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(navigateCmd, backCmd, forwardCmd, reloadCmd)
}

var navigateCmd = &cobra.Command{
	Use:   "navigate <url>",
	Short: "Navigate to a URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		resp, err := client.Navigate(context.Background(), &pb.NavigateRequest{
			SessionId:  sessionID,
			Url:        args[0],
			WaitStable: true,
		})
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s - %s\n", resp.GetTitle(), resp.GetUrl())
		return nil
	},
}

var backCmd = &cobra.Command{
	Use:   "back",
	Short: "Go back in browser history",
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

		_, err = client.GoBack(context.Background(), &pb.SessionRequest{SessionId: sessionID})
		if err != nil {
			return fmt.Errorf("scout: back: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "navigated back")
		return nil
	},
}

var forwardCmd = &cobra.Command{
	Use:   "forward",
	Short: "Go forward in browser history",
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

		_, err = client.GoForward(context.Background(), &pb.SessionRequest{SessionId: sessionID})
		if err != nil {
			return fmt.Errorf("scout: forward: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "navigated forward")
		return nil
	},
}

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload the current page",
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

		_, err = client.Reload(context.Background(), &pb.SessionRequest{SessionId: sessionID})
		if err != nil {
			return fmt.Errorf("scout: reload: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "page reloaded")
		return nil
	},
}
