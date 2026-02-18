package main

import (
	"context"
	"fmt"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(harCmd)
	harCmd.AddCommand(harStartCmd, harStopCmd, harExportCmd)

	harStartCmd.Flags().Bool("capture-body", false, "capture response bodies")
}

var harCmd = &cobra.Command{
	Use:   "har",
	Short: "Manage HAR network recording",
}

var harStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start recording network traffic",
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

		captureBody, _ := cmd.Flags().GetBool("capture-body")

		_, err = client.StartRecording(context.Background(), &pb.RecordingRequest{
			SessionId:   sessionID,
			CaptureBody: captureBody,
		})
		if err != nil {
			return fmt.Errorf("scout: start recording: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "recording started")
		return nil
	},
}

var harStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop recording network traffic",
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

		_, err = client.StopRecording(context.Background(), &pb.SessionRequest{SessionId: sessionID})
		if err != nil {
			return fmt.Errorf("scout: stop recording: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "recording stopped")
		return nil
	},
}

var harExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export recorded HAR data",
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

		resp, err := client.ExportHAR(context.Background(), &pb.SessionRequest{SessionId: sessionID})
		if err != nil {
			return fmt.Errorf("scout: export har: %w", err)
		}

		defaultName := fmt.Sprintf("capture_%d.har", time.Now().Unix())
		filename, err := writeOutput(cmd, resp.GetData(), defaultName)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "HAR exported: %s (%d entries)\n", filename, resp.GetEntryCount())
		return nil
	},
}
