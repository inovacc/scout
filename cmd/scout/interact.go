package main

import (
	"context"
	"fmt"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(clickCmd, typeCmd, selectCmd, hoverCmd, focusCmd, clearCmd, keyCmd)

	typeCmd.Flags().Bool("clear", false, "clear field before typing")
}

var clickCmd = &cobra.Command{
	Use:   "click <selector>",
	Short: "Click an element",
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

		_, err = client.Click(context.Background(), &pb.ElementRequest{
			SessionId: sessionID,
			Selector:  args[0],
		})
		if err != nil {
			return fmt.Errorf("scout: click: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "clicked")
		return nil
	},
}

var typeCmd = &cobra.Command{
	Use:   "type <selector> <text>",
	Short: "Type text into an element",
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

		clearFirst, _ := cmd.Flags().GetBool("clear")

		_, err = client.Type(context.Background(), &pb.TypeRequest{
			SessionId:  sessionID,
			Selector:   args[0],
			Text:       args[1],
			ClearFirst: clearFirst,
		})
		if err != nil {
			return fmt.Errorf("scout: type: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "typed")
		return nil
	},
}

var selectCmd = &cobra.Command{
	Use:   "select <selector> <value>",
	Short: "Select an option in a dropdown",
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

		_, err = client.SelectOption(context.Background(), &pb.SelectRequest{
			SessionId: sessionID,
			Selector:  args[0],
			Value:     args[1],
		})
		if err != nil {
			return fmt.Errorf("scout: select: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "selected")
		return nil
	},
}

var hoverCmd = &cobra.Command{
	Use:   "hover <selector>",
	Short: "Hover over an element",
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

		_, err = client.Hover(context.Background(), &pb.ElementRequest{
			SessionId: sessionID,
			Selector:  args[0],
		})
		if err != nil {
			return fmt.Errorf("scout: hover: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "hovered")
		return nil
	},
}

var focusCmd = &cobra.Command{
	Use:   "focus <selector>",
	Short: "Focus an element",
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

		// Use Eval to focus — no dedicated gRPC RPC for focus
		_, err = client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    fmt.Sprintf(`document.querySelector(%q).focus()`, args[0]),
		})
		if err != nil {
			return fmt.Errorf("scout: focus: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "focused")
		return nil
	},
}

var clearCmd = &cobra.Command{
	Use:   "clear <selector>",
	Short: "Clear an input element",
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

		// Use Type with clear_first and empty text
		_, err = client.Type(context.Background(), &pb.TypeRequest{
			SessionId:  sessionID,
			Selector:   args[0],
			Text:       "",
			ClearFirst: true,
		})
		if err != nil {
			return fmt.Errorf("scout: clear: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "cleared")
		return nil
	},
}

var keyCmd = &cobra.Command{
	Use:   "key <key>",
	Short: "Press a key (Enter, Tab, Escape, etc.)",
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

		_, err = client.PressKey(context.Background(), &pb.KeyRequest{
			SessionId: sessionID,
			Key:       args[0],
		})
		if err != nil {
			return fmt.Errorf("scout: key: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "key pressed")
		return nil
	},
}
