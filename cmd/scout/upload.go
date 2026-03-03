package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

func init() {
	rootCmd.AddCommand(uploadCmd)
	uploadCmd.AddCommand(uploadAuthCmd, uploadFileCmd, uploadStatusCmd)

	uploadAuthCmd.Flags().String("sink", "gdrive", "upload sink: gdrive, onedrive")
	uploadAuthCmd.Flags().String("client-id", "", "OAuth2 client ID")
	uploadAuthCmd.Flags().String("client-secret", "", "OAuth2 client secret")
	uploadAuthCmd.Flags().String("folder-id", "", "destination folder ID (GDrive)")
	uploadAuthCmd.Flags().String("folder-path", "", "destination folder path (OneDrive)")

	uploadFileCmd.Flags().Bool("json", false, "output result as JSON")
}

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload files to Google Drive or OneDrive",
}

var uploadAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with a cloud storage provider",
	RunE: func(cmd *cobra.Command, _ []string) error {
		sinkStr, _ := cmd.Flags().GetString("sink")
		clientID, _ := cmd.Flags().GetString("client-id")
		clientSecret, _ := cmd.Flags().GetString("client-secret")
		folderID, _ := cmd.Flags().GetString("folder-id")
		folderPath, _ := cmd.Flags().GetString("folder-path")

		if clientID == "" {
			clientID = os.Getenv("SCOUT_OAUTH_CLIENT_ID")
		}

		if clientSecret == "" {
			clientSecret = os.Getenv("SCOUT_OAUTH_CLIENT_SECRET")
		}

		if clientID == "" || clientSecret == "" {
			return fmt.Errorf("--client-id and --client-secret required (or set SCOUT_OAUTH_CLIENT_ID/SCOUT_OAUTH_CLIENT_SECRET)")
		}

		sink := scout.UploadSink(sinkStr)

		oauthCfg := scout.UploadOAuthConfig(sink, clientID, clientSecret, "urn:ietf:wg:oauth:2.0:oob")
		if oauthCfg == nil {
			return fmt.Errorf("unsupported sink: %s", sinkStr)
		}

		authURL := oauthCfg.AuthCodeURL("state", oauth2.AccessTypeOffline)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Open this URL in your browser:\n\n  %s\n\n", authURL)
		_, _ = fmt.Fprint(cmd.OutOrStdout(), "Enter authorization code: ")

		var code string
		if _, err := fmt.Fscanln(os.Stdin, &code); err != nil {
			return fmt.Errorf("scout: upload auth: read code: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		token, err := oauthCfg.Exchange(ctx, code)
		if err != nil {
			return fmt.Errorf("scout: upload auth: exchange token: %w", err)
		}

		cfg := &scout.UploadConfig{
			Sink:       sink,
			Token:      token,
			FolderID:   folderID,
			FolderPath: folderPath,
		}

		if err := scout.SaveUploadConfig(cfg); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Authenticated with %s. Config saved to ~/.scout/upload.json\n", sinkStr)

		return nil
	},
}

var uploadFileCmd = &cobra.Command{
	Use:   "file <path>...",
	Short: "Upload one or more files to the configured cloud sink",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := scout.LoadUploadConfig()
		if err != nil {
			return fmt.Errorf("no upload config found — run 'scout upload auth' first: %w", err)
		}

		uploader := scout.NewUploader(cfg)
		jsonOut, _ := cmd.Flags().GetBool("json")

		ctx := context.Background()

		var results []scout.UploadResult

		for _, path := range args {
			result, err := uploader.UploadFile(ctx, path)
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "ERROR uploading %s: %v\n", path, err)
				continue
			}

			results = append(results, *result)

			if !jsonOut {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Uploaded: %s → %s (%s)\n", result.FileName, result.URL, result.Sink)
			}
		}

		if jsonOut {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			if err := enc.Encode(results); err != nil {
				return fmt.Errorf("scout: upload: encode: %w", err)
			}
		}

		return nil
	},
}

var uploadStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show upload configuration status",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := scout.LoadUploadConfig()
		if err != nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Not configured. Run 'scout upload auth' to set up.")
			return nil
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Sink: %s\n", cfg.Sink)

		if cfg.FolderID != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Folder ID: %s\n", cfg.FolderID)
		}

		if cfg.FolderPath != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Folder Path: %s\n", cfg.FolderPath)
		}

		if cfg.Token != nil {
			if cfg.Token.Expiry.IsZero() {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Token: valid (no expiry)")
			} else if cfg.Token.Expiry.After(time.Now()) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Token: valid (expires %s)\n", cfg.Token.Expiry.Format(time.RFC3339))
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Token: expired (%s) — re-run 'scout upload auth'\n", cfg.Token.Expiry.Format(time.RFC3339))
			}
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Token: missing")
		}

		return nil
	},
}
