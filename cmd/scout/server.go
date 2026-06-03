package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/grpc/server"
	identity2 "github.com/inovacc/scout/pkg/scout/identity"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().Int("port", 9551, "gRPC server port")
	serverCmd.Flags().String("host", "", "bind host (default: loopback for --insecure, all interfaces for mTLS)")
	serverCmd.Flags().Bool("reflection", false, "enable gRPC reflection (exposes the service schema)")
	serverCmd.Flags().Bool("insecure", false, "disable mTLS (no authentication)")
	serverCmd.Flags().String("pairing-token", "", "out-of-band token required to pair a new device (env SCOUT_PAIRING_TOKEN); a random token is generated and printed if unset")
	// --idle-timeout inherited from root persistent flags
}

// isLoopbackHost reports whether host refers only to the local machine.
func isLoopbackHost(host string) bool {
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the gRPC browser automation server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")
		enableReflection, _ := cmd.Flags().GetBool("reflection")
		insecureMode, _ := cmd.Flags().GetBool("insecure")

		// Default the insecure (no-auth) server to loopback so it is never
		// exposed to the network. The mTLS server keeps binding all interfaces
		// (it is authenticated and meant for LAN device pairing).
		if host == "" && insecureMode {
			host = "127.0.0.1"
		}
		if insecureMode && !isLoopbackHost(host) {
			_, _ = fmt.Fprintln(os.Stderr, "warning: --insecure server is binding a non-loopback address with NO authentication; any host that can reach it can control this browser")
		}

		addr := net.JoinHostPort(host, strconv.Itoa(port))

		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("scout: listen on %s: %w", addr, err)
		}

		msgOpts := []grpc.ServerOption{
			grpc.MaxRecvMsgSize(64 * 1024 * 1024),
			grpc.MaxSendMsgSize(64 * 1024 * 1024),
		}

		var (
			grpcServer  *grpc.Server
			scoutServer *server.ScoutServer
			deviceID    string
		)

		if insecureMode {
			grpcServer = grpc.NewServer(msgOpts...)
			scoutServer = server.New()
		} else {
			dir, err := scoutDir()
			if err != nil {
				return err
			}

			id, err := identity2.LoadOrGenerate(filepath.Join(dir, "identity"))
			if err != nil {
				return fmt.Errorf("scout: load identity: %w", err)
			}

			deviceID = id.DeviceID

			trustStore, err := identity2.NewTrustStore(filepath.Join(dir, "trusted"))
			if err != nil {
				return fmt.Errorf("scout: trust store: %w", err)
			}

			grpcServer, scoutServer, err = server.NewTLSServer(id, trustStore, msgOpts...)
			if err != nil {
				return fmt.Errorf("scout: create TLS server: %w", err)
			}
		}

		pairingAddr := ""
		pairingToken := ""
		info := server.ServerInfo{
			DeviceID:   deviceID,
			ListenAddr: addr,
			Insecure:   insecureMode,
			LocalIPs:   server.GetLocalIPs(),
		}

		// Print initial table
		var displayMu sync.Mutex

		refreshDisplay := func() {
			displayMu.Lock()
			defer displayMu.Unlock()

			_, _ = fmt.Fprint(os.Stdout, "\033[2J\033[H") // clear screen + cursor home
			info.PairingAddr = pairingAddr
			info.PairingToken = pairingToken
			info.TotalSessions, _ = scoutServer.Stats()
			info.Events = scoutServer.Events()
			server.PrintServerTable(os.Stdout, info, scoutServer.Peers())
		}
		printTable := func(_ []server.ConnectedPeer) {
			refreshDisplay()
		}

		scoutServer.OnPeerChange = printTable
		scoutServer.OnStatsChange = refreshDisplay

		pb.RegisterScoutServiceServer(grpcServer, scoutServer)

		if enableReflection {
			reflection.Register(grpcServer)
		}

		// Register standard gRPC health service.
		healthSrv := health.NewServer()
		healthSrv.SetServingStatus("scout.ScoutService", healthpb.HealthCheckResponse_SERVING)
		healthpb.RegisterHealthServer(grpcServer, healthSrv)

		// Start pairing listener on port+1 when mTLS is enabled.
		var pairingGRPC *grpc.Server

		if !insecureMode {
			dir, _ := scoutDir()
			id, _ := identity2.LoadOrGenerate(filepath.Join(dir, "identity"))
			trustStore, _ := identity2.NewTrustStore(filepath.Join(dir, "trusted"))

			// Resolve the out-of-band pairing token: flag > env > random.
			pairingToken, _ = cmd.Flags().GetString("pairing-token")
			if pairingToken == "" {
				pairingToken = os.Getenv("SCOUT_PAIRING_TOKEN")
			}
			if pairingToken == "" {
				pairingToken, err = server.GeneratePairingToken()
				if err != nil {
					return fmt.Errorf("scout: generate pairing token: %w", err)
				}
			}

			pairingAddr = fmt.Sprintf(":%d", port+1)

			pairingLis, err := net.Listen("tcp", pairingAddr)
			if err != nil {
				return fmt.Errorf("scout: listen pairing on %s: %w", pairingAddr, err)
			}

			pairingGRPC = grpc.NewServer()
			pairingSrv := server.NewPairingServer(id, trustStore, pairingToken)
			pairingSrv.OnPaired = func(deviceID string) {
				scoutServer.NotifyPeerChange()
			}
			pb.RegisterPairingServiceServer(pairingGRPC, pairingSrv)

			go func() {
				_ = pairingGRPC.Serve(pairingLis)
			}()
		}

		// Wire idle timeout.
		idleTimeout, _ := cmd.Flags().GetDuration("idle-timeout")
		scoutServer.IdleTimeout = idleTimeout
		scoutServer.OnIdleShutdown = func() {
			_, _ = fmt.Fprintln(os.Stdout, "\nidle timeout reached, shutting down gRPC server...")

			func() {
				defer func() {
					if r := recover(); r != nil {
						_, _ = fmt.Fprintf(os.Stdout, "session teardown panicked: %v\n", r)
					}
				}()
				scoutServer.DestroyAllSessions()
			}()

			if pairingGRPC != nil {
				pairingGRPC.GracefulStop()
			}

			grpcServer.GracefulStop()
		}

		scoutServer.StartIdleTimer()
		defer scoutServer.StopIdleTimer()

		printTable(nil)

		// Reap prior-instance session orphans left by a crashed or
		// force-killed previous daemon before we start serving. Map is
		// empty at boot, so this only touches on-disk orphans.
		if killed := scoutServer.Reconcile(); killed > 0 {
			_, _ = fmt.Fprintf(os.Stdout, "reconcile: reaped %d orphaned browser process(es)\n", killed)
		}

		// Graceful shutdown
		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			_, _ = fmt.Fprintln(os.Stdout, "\nshutting down gRPC server...")

			func() {
				defer func() {
					if r := recover(); r != nil {
						_, _ = fmt.Fprintf(os.Stdout, "session teardown panicked: %v\n", r)
					}
				}()
				scoutServer.DestroyAllSessions()
			}()

			if pairingGRPC != nil {
				pairingGRPC.GracefulStop()
			}

			grpcServer.GracefulStop()
		}()

		if err := grpcServer.Serve(lis); err != nil {
			return fmt.Errorf("scout: serve: %w", err)
		}

		return nil
	},
}
