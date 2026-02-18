package main

import (
	"context"
	"fmt"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(titleCmd, urlCmd, textCmd, attrCmd, evalCmd, htmlCmd)

	htmlCmd.Flags().String("selector", "", "CSS selector (defaults to entire page)")
}

var titleCmd = &cobra.Command{
	Use:   "title",
	Short: "Get the page title",
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

		resp, err := client.GetTitle(context.Background(), &pb.SessionRequest{SessionId: sessionID})
		if err != nil {
			return fmt.Errorf("scout: title: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetText())
		return nil
	},
}

var urlCmd = &cobra.Command{
	Use:   "url",
	Short: "Get the current page URL",
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

		resp, err := client.GetURL(context.Background(), &pb.SessionRequest{SessionId: sessionID})
		if err != nil {
			return fmt.Errorf("scout: url: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetText())
		return nil
	},
}

var textCmd = &cobra.Command{
	Use:   "text <selector>",
	Short: "Get text content of an element",
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

		resp, err := client.GetText(context.Background(), &pb.ElementRequest{
			SessionId: sessionID,
			Selector:  args[0],
		})
		if err != nil {
			return fmt.Errorf("scout: text: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetText())
		return nil
	},
}

var attrCmd = &cobra.Command{
	Use:   "attr <selector> <attribute>",
	Short: "Get an attribute value from an element",
	Args:  cobra.ExactArgs(2),
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

		resp, err := client.GetAttribute(context.Background(), &pb.AttributeRequest{
			SessionId: sessionID,
			Selector:  args[0],
			Attribute: args[1],
		})
		if err != nil {
			return fmt.Errorf("scout: attr: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetText())
		return nil
	},
}

var evalCmd = &cobra.Command{
	Use:   "eval <javascript>",
	Short: "Evaluate JavaScript in the page context",
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

		resp, err := client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    args[0],
		})
		if err != nil {
			return fmt.Errorf("scout: eval: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetResult())
		return nil
	},
}

var htmlCmd = &cobra.Command{
	Use:   "html",
	Short: "Get HTML content of the page or an element",
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

		selector, _ := cmd.Flags().GetString("selector")
		if selector == "" {
			selector = "html"
		}

		// Use Eval to get innerHTML
		resp, err := client.Eval(context.Background(), &pb.EvalRequest{
			SessionId: sessionID,
			Script:    fmt.Sprintf(`document.querySelector(%q).innerHTML`, selector),
		})
		if err != nil {
			return fmt.Errorf("scout: html: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.GetResult())
		return nil
	},
}
