package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/inovacc/scout/internal/engine"
	"github.com/spf13/cobra"
)

var mobileCmd = &cobra.Command{
	Use:   "mobile",
	Short: "Mobile browser automation via ADB",
	Long:  "Control Chrome on Android devices via ADB port forwarding and CDP.",
}

var mobileDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List connected Android devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		adbPath, _ := cmd.Flags().GetString("adb")
		devices, err := engine.ListADBDevices(context.Background(), adbPath)
		if err != nil {
			return err
		}

		if len(devices) == 0 {
			_, _ = fmt.Fprintln(os.Stderr, "No devices found. Ensure USB debugging is enabled.")
			return nil
		}

		asJSON, _ := cmd.Flags().GetBool("json")
		if asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(devices)
		}

		for _, d := range devices {
			model := d.Model
			if model == "" {
				model = "unknown"
			}
			_, _ = fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", d.Serial, d.State, model)
		}

		return nil
	},
}

var mobileConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to Chrome on an Android device",
	Long:  "Set up ADB port forwarding and launch an interactive session with mobile Chrome.",
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID, _ := cmd.Flags().GetString("device")
		adbPath, _ := cmd.Flags().GetString("adb")
		port, _ := cmd.Flags().GetInt("port")
		url, _ := cmd.Flags().GetString("url")

		cfg := engine.MobileConfig{
			DeviceID: deviceID,
			ADBPath:  adbPath,
			CDPPort:  port,
		}

		_, _ = fmt.Fprintln(os.Stderr, "Setting up ADB port forwarding...")

		b, err := engine.New(
			engine.WithMobile(cfg),
			engine.WithHeadless(false),
		)
		if err != nil {
			return err
		}

		defer func() { _ = b.Close() }()

		if url != "" {
			page, pageErr := b.NewPage(url)
			if pageErr != nil {
				return pageErr
			}
			_ = page.WaitLoad()
			title, _ := page.Title()
			_, _ = fmt.Fprintf(os.Stdout, "Connected to mobile Chrome — %s\n", title)
		} else {
			_, _ = fmt.Fprintln(os.Stdout, "Connected to mobile Chrome. Press Ctrl+C to disconnect.")
		}

		// Wait for interrupt
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		<-ctx.Done()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(mobileCmd)
	mobileCmd.AddCommand(mobileDevicesCmd, mobileConnectCmd)

	mobileCmd.PersistentFlags().String("adb", "", "Path to adb binary (default: adb in PATH)")

	mobileDevicesCmd.Flags().Bool("json", false, "Output as JSON")

	mobileConnectCmd.Flags().StringP("device", "d", "", "ADB device serial (from 'scout mobile devices')")
	mobileConnectCmd.Flags().Int("port", 9222, "Local port for CDP forwarding")
	mobileConnectCmd.Flags().String("url", "", "URL to navigate to after connecting")
}
