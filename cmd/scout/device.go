package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/pkg/discovery"
	"github.com/inovacc/scout/pkg/identity"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	rootCmd.AddCommand(deviceCmd)
	deviceCmd.AddCommand(deviceIDCmd, deviceTrustCmd, deviceListCmd, deviceRemoveCmd, deviceDiscoverCmd, devicePairCmd)

	devicePairCmd.Flags().String("addr", "", "address of the remote pairing endpoint (host:port)")
	devicePairCmd.Flags().String("server-id", "", "expected server device ID (skip interactive confirmation)")
	_ = devicePairCmd.MarkFlagRequired("addr")
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

var devicePairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Pair with a remote scout server to exchange mTLS certificates",
	Long:  "Connects to a remote server's pairing endpoint (insecure), sends this device's certificate, and receives the server's certificate. After pairing, mTLS connections work automatically.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		expectedID, _ := cmd.Flags().GetString("server-id")

		dir, err := scoutDir()
		if err != nil {
			return err
		}

		id, err := identity.LoadOrGenerate(filepath.Join(dir, "identity"))
		if err != nil {
			return fmt.Errorf("scout: load identity: %w", err)
		}

		trustStore, err := identity.NewTrustStore(filepath.Join(dir, "trusted"))
		if err != nil {
			return fmt.Errorf("scout: trust store: %w", err)
		}

		// Connect to the pairing endpoint (insecure).
		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return fmt.Errorf("scout: connect to pairing endpoint %s: %w", addr, err)
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewPairingServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := client.Pair(ctx, &pb.PairRequest{
			DeviceId: id.DeviceID,
			CertDer:  id.Certificate.Certificate[0],
		})
		if err != nil {
			return fmt.Errorf("scout: pair: %w", err)
		}

		serverID := resp.GetServerDeviceId()

		// Verify server identity.
		if expectedID != "" {
			if serverID != expectedID {
				return fmt.Errorf("scout: server ID mismatch: expected %s, got %s",
					identity.ShortID(expectedID), identity.ShortID(serverID))
			}
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Server device ID: %s\nTrust this server? [y/N] ", serverID)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				return fmt.Errorf("scout: pairing cancelled by user")
			}
		}

		// Store the server's certificate.
		if err := trustStore.Trust(serverID, resp.GetServerCertDer()); err != nil {
			return fmt.Errorf("scout: store server cert: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Paired with %s successfully.\n", identity.ShortID(serverID))
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
