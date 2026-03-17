package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/inovacc/scout/pkg/scout/proxy"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.AddCommand(proxyStartCmd)
	proxyCmd.AddCommand(proxyRoutesCmd)

	proxyStartCmd.Flags().StringP("file", "f", "routes.yaml", "routes config file")
	proxyStartCmd.Flags().StringP("port", "p", "8080", "listen port")
	proxyStartCmd.Flags().String("addr", "", "listen address (overrides --port)")
}

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "HTTP reverse proxy turning websites into REST/JSON endpoints",
	Long:  "Run an API proxy that scrapes websites on demand and returns structured JSON via HTTP endpoints.",
}

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the API proxy server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")
		port, _ := cmd.Flags().GetString("port")
		addr, _ := cmd.Flags().GetString("addr")

		if addr == "" {
			addr = ":" + port
		}

		cfg, err := proxy.LoadConfig(file)
		if err != nil {
			return fmt.Errorf("scout: proxy: %w", err)
		}

		srv, err := proxy.New(cfg)
		if err != nil {
			return fmt.Errorf("scout: proxy: %w", err)
		}

		defer func() { _ = srv.Close() }()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-sigCh
			signal.Stop(sigCh)
			_ = srv.Close()
		}()

		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "proxy listening on %s (%d routes)\n", addr, len(cfg.Routes))

		return srv.ListenAndServe(addr)
	},
}

var proxyRoutesCmd = &cobra.Command{
	Use:   "routes",
	Short: "List configured proxy routes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Parent().Flags().GetString("file")
		if file == "" {
			file = "routes.yaml"
		}

		cfg, err := proxy.LoadConfig(file)
		if err != nil {
			return fmt.Errorf("scout: proxy: %w", err)
		}

		for _, r := range cfg.Routes {
			method := r.Method
			if method == "" {
				method = "GET"
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-6s %-30s → %s\n", method, r.Path, r.Target)
		}

		return nil
	},
}
