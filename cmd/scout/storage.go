package main

import (
	"context"
	"fmt"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(storageCmd)
	storageCmd.AddCommand(storageGetCmd, storageSetCmd, storageListCmd, storageClearCmd)

	storageCmd.PersistentFlags().Bool("session-storage", false, "use sessionStorage instead of localStorage")
}

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage browser web storage",
}

func storageType(cmd *cobra.Command) string {
	useSession, _ := cmd.Flags().GetBool("session-storage")
	if useSession {
		return "sessionStorage"
	}

	return "localStorage"
}

var storageGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a value from web storage",
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

		st := storageType(cmd)
		resp, err := client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    fmt.Sprintf(`%s.getItem(%q)`, st, args[0]),
		})
		if err != nil {
			return fmt.Errorf("scout: storage get: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetResult())
		return nil
	},
}

var storageSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a value in web storage",
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

		st := storageType(cmd)
		_, err = client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    fmt.Sprintf(`%s.setItem(%q, %q)`, st, args[0], args[1]),
		})
		if err != nil {
			return fmt.Errorf("scout: storage set: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "stored")
		return nil
	},
}

var storageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all keys in web storage",
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

		st := storageType(cmd)
		resp, err := client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    fmt.Sprintf(`JSON.stringify(Object.keys(%s))`, st),
		})
		if err != nil {
			return fmt.Errorf("scout: storage list: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetResult())
		return nil
	},
}

var storageClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all entries in web storage",
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

		st := storageType(cmd)
		_, err = client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    fmt.Sprintf(`%s.clear()`, st),
		})
		if err != nil {
			return fmt.Errorf("scout: storage clear: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "storage cleared")
		return nil
	},
}
