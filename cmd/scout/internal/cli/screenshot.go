package cli

import (
	"context"
	"fmt"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(screenshotCmd, pdfCmd)

	screenshotCmd.Flags().Bool("full", false, "capture full page")
	screenshotCmd.Flags().String("format", "png", "image format (png, jpeg)")
	screenshotCmd.Flags().Int("quality", 80, "jpeg quality (0-100)")
}

var screenshotCmd = &cobra.Command{
	Use:   "screenshot",
	Short: "Take a screenshot of the current page",
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

		fullPage, _ := cmd.Flags().GetBool("full")
		imgFormat, _ := cmd.Flags().GetString("format")
		quality, _ := cmd.Flags().GetInt("quality")

		resp, err := client.Screenshot(context.Background(), &pb.ScreenshotRequest{
			SessionId: sessionID,
			FullPage:  fullPage,
			Format:    imgFormat,
			Quality:   int32(quality),
		})
		if err != nil {
			return fmt.Errorf("scout: screenshot: %w", err)
		}

		defaultName := fmt.Sprintf("screenshot_%d.%s", time.Now().Unix(), imgFormat)
		filename, err := writeOutput(cmd, resp.GetData(), defaultName)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved to %s (%d bytes)\n", filename, len(resp.GetData()))
		return nil
	},
}

var pdfCmd = &cobra.Command{
	Use:   "pdf",
	Short: "Generate a PDF of the current page",
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

		resp, err := client.PDF(context.Background(), &pb.SessionRequest{SessionId: sessionID})
		if err != nil {
			return fmt.Errorf("scout: pdf: %w", err)
		}

		defaultName := fmt.Sprintf("page_%d.pdf", time.Now().Unix())
		filename, err := writeOutput(cmd, resp.GetData(), defaultName)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved to %s (%d bytes)\n", filename, len(resp.GetData()))
		return nil
	},
}
