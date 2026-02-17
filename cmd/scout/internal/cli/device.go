package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/inovacc/scout/pkg/discovery"
	"github.com/inovacc/scout/pkg/identity"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(deviceCmd)
	deviceCmd.AddCommand(deviceIDCmd, deviceTrustCmd, deviceListCmd, deviceRemoveCmd, deviceDiscoverCmd)
}

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Manage device identity and trust",
}

var deviceIDCmd = &cobra.Command{
	Use:   "id",
	Short: "Print this instance's device ID",
	RunE: func(cmd *cobra.Command, _ []string) error {
		dir, err := scoutDir()
		if err != nil {
			return err
		}

		id, err := identity.LoadOrGenerate(filepath.Join(dir, "identity"))
		if err != nil {
			return fmt.Errorf("scout: load identity: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), id.DeviceID)
		return nil
	},
}

var deviceTrustCmd = &cobra.Command{
	Use:   "trust <device-id>",
	Short: "Add a device to the trusted set",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]
		if !identity.ValidateDeviceID(deviceID) {
			return fmt.Errorf("scout: invalid device ID: %s", deviceID)
		}

		dir, err := scoutDir()
		if err != nil {
			return err
		}

		ts, err := identity.NewTrustStore(filepath.Join(dir, "trusted"))
		if err != nil {
			return err
		}

		// Trust with empty cert — the actual cert will be verified on first mTLS handshake.
		// For now, just create the trust entry so the device ID is recognized.
		if err := ts.Trust(deviceID, nil); err != nil {
			return fmt.Errorf("scout: trust device: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Trusted: %s\n", deviceID)
		return nil
	},
}

var deviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List trusted devices",
	RunE: func(cmd *cobra.Command, _ []string) error {
		dir, err := scoutDir()
		if err != nil {
			return err
		}

		ts, err := identity.NewTrustStore(filepath.Join(dir, "trusted"))
		if err != nil {
			return err
		}

		devices, err := ts.List()
		if err != nil {
			return err
		}

		if len(devices) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No trusted devices.")
			return nil
		}

		for _, d := range devices {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s  (trusted %s)\n",
				d.DeviceID, d.TrustedAt.Format(time.RFC3339))
		}

		return nil
	},
}

var deviceRemoveCmd = &cobra.Command{
	Use:   "remove <device-id>",
	Short: "Remove a device from the trusted set",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := scoutDir()
		if err != nil {
			return err
		}

		ts, err := identity.NewTrustStore(filepath.Join(dir, "trusted"))
		if err != nil {
			return err
		}

		if err := ts.Remove(args[0]); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed: %s\n", args[0])
		return nil
	},
}

var deviceDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Scan local network for scout instances via mDNS",
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Scanning for scout instances (5s)...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		peers, err := discovery.Discover(ctx)
		if err != nil {
			return fmt.Errorf("scout: discover: %w", err)
		}

		found := 0
		for peer := range peers {
			found++
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s:%d",
				peer.DeviceID, peer.Host, peer.Port)
			if len(peer.Addrs) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  (%s)", peer.Addrs[0])
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}

		if found == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No scout instances found.")
		}

		return nil
	},
}
