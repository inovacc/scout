package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

var vpnCmd = &cobra.Command{
	Use:   "vpn",
	Short: "VPN and proxy management",
}

var vpnStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current VPN/proxy status",
	RunE: func(cmd *cobra.Command, _ []string) error {
		provider, err := vpnProviderFromFlags(cmd)
		if err != nil {
			return err
		}

		st, err := provider.Status(context.Background())
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		w := cmd.OutOrStdout()

		if format == "json" {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")

			return enc.Encode(st)
		}

		_, _ = fmt.Fprintf(w, "Provider:  %s\n", provider.Name())

		_, _ = fmt.Fprintf(w, "Connected: %v\n", st.Connected)
		if st.Connection != nil {
			_, _ = fmt.Fprintf(w, "Server:    %s\n", st.Connection.Server.Host)
			_, _ = fmt.Fprintf(w, "Protocol:  %s\n", st.Connection.Protocol)
			_, _ = fmt.Fprintf(w, "Port:      %d\n", st.Connection.Port)
		}

		if st.PublicIP != "" {
			_, _ = fmt.Fprintf(w, "Public IP: %s\n", st.PublicIP)
		}

		return nil
	},
}

var vpnConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to VPN/proxy",
	RunE: func(cmd *cobra.Command, _ []string) error {
		provider, err := vpnProviderFromFlags(cmd)
		if err != nil {
			return err
		}

		country, _ := cmd.Flags().GetString("country")

		conn, err := provider.Connect(context.Background(), country)
		if err != nil {
			return err
		}

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "Connected to %s via %s:%d\n", conn.Server.Host, conn.Protocol, conn.Port)

		return nil
	},
}

var vpnDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect from VPN/proxy",
	RunE: func(cmd *cobra.Command, _ []string) error {
		provider, err := vpnProviderFromFlags(cmd)
		if err != nil {
			return err
		}

		if err := provider.Disconnect(context.Background()); err != nil {
			return err
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Disconnected")

		return nil
	},
}

var vpnServersCmd = &cobra.Command{
	Use:   "servers",
	Short: "List available VPN servers",
	RunE: func(cmd *cobra.Command, _ []string) error {
		provider, err := vpnProviderFromFlags(cmd)
		if err != nil {
			return err
		}

		servers, err := provider.Servers(context.Background())
		if err != nil {
			return err
		}

		country, _ := cmd.Flags().GetString("country")
		format, _ := cmd.Flags().GetString("format")
		w := cmd.OutOrStdout()

		var filtered []scout.VPNServer

		for _, s := range servers {
			if country == "" || s.Country == country {
				filtered = append(filtered, s)
			}
		}

		if format == "json" {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")

			return enc.Encode(filtered)
		}

		for _, s := range filtered {
			_, _ = fmt.Fprintf(w, "%-30s  %s  %s  load=%d%%\n", s.Host, s.Country, s.City, s.Load)
		}

		if len(filtered) == 0 {
			_, _ = fmt.Fprintln(w, "No servers found")
		}

		return nil
	},
}

func init() {
	vpnConnectCmd.Flags().String("country", "", "country code (ISO 2-letter, e.g. us)")
	vpnServersCmd.Flags().String("country", "", "filter by country code")

	vpnCmd.PersistentFlags().String("proxy-host", "", "proxy host for direct proxy provider")
	vpnCmd.PersistentFlags().Int("proxy-port", 1080, "proxy port for direct proxy provider")
	vpnCmd.PersistentFlags().String("proxy-scheme", "socks5", "proxy scheme (socks5, https)")
	vpnCmd.PersistentFlags().String("proxy-user", "", "proxy auth username")
	vpnCmd.PersistentFlags().String("proxy-pass", "", "proxy auth password")

	vpnCmd.AddCommand(vpnStatusCmd, vpnConnectCmd, vpnDisconnectCmd, vpnServersCmd)
	rootCmd.AddCommand(vpnCmd)
}

// vpnProviderFromFlags builds a DirectProxy VPN provider from CLI flags.
func vpnProviderFromFlags(cmd *cobra.Command) (scout.VPNProvider, error) {
	host, _ := cmd.Flags().GetString("proxy-host")
	if host == "" {
		return nil, fmt.Errorf("scout: vpn: --proxy-host is required")
	}

	port, _ := cmd.Flags().GetInt("proxy-port")
	scheme, _ := cmd.Flags().GetString("proxy-scheme")
	user, _ := cmd.Flags().GetString("proxy-user")
	pass, _ := cmd.Flags().GetString("proxy-pass")

	var opts []scout.DirectProxyOption
	if scheme != "" {
		opts = append(opts, scout.WithDirectProxyScheme(scheme))
	}

	if user != "" {
		opts = append(opts, scout.WithDirectProxyAuth(user, pass))
	}

	return scout.NewDirectProxy(host, port, opts...), nil
}
