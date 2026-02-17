package cli

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/grpc/server"
	"github.com/inovacc/scout/pkg/identity"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().Int("port", 50051, "gRPC server port")
	serverCmd.Flags().Bool("reflection", true, "enable gRPC reflection")
	serverCmd.Flags().Bool("insecure", false, "disable mTLS (no authentication)")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the gRPC browser automation server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		port, _ := cmd.Flags().GetInt("port")
		enableReflection, _ := cmd.Flags().GetBool("reflection")
		insecureMode, _ := cmd.Flags().GetBool("insecure")

		addr := fmt.Sprintf(":%d", port)
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

			id, err := identity.LoadOrGenerate(filepath.Join(dir, "identity"))
			if err != nil {
				return fmt.Errorf("scout: load identity: %w", err)
			}

			deviceID = id.DeviceID

			trustStore, err := identity.NewTrustStore(filepath.Join(dir, "trusted"))
			if err != nil {
				return fmt.Errorf("scout: trust store: %w", err)
			}

			grpcServer, scoutServer, err = server.NewTLSServer(id, trustStore, msgOpts...)
			if err != nil {
				return fmt.Errorf("scout: create TLS server: %w", err)
			}
		}

		pairingAddr := ""
		info := server.ServerInfo{
			DeviceID:   deviceID,
			ListenAddr: addr,
			Insecure:   insecureMode,
			LocalIPs:   server.GetLocalIPs(),
		}

		// Print initial table
		var displayMu sync.Mutex
		printTable := func(peers []server.ConnectedPeer) {
			displayMu.Lock()
			defer displayMu.Unlock()
			_, _ = fmt.Fprint(os.Stdout, "\033[2J\033[H") // clear screen + cursor home
			info.PairingAddr = pairingAddr
			server.PrintServerTable(os.Stdout, info, peers)
		}

		scoutServer.OnPeerChange = printTable

		pb.RegisterScoutServiceServer(grpcServer, scoutServer)

		if enableReflection {
			reflection.Register(grpcServer)
		}

		// Start pairing listener on port+1 when mTLS is enabled.
		var pairingGRPC *grpc.Server
		if !insecureMode {
			dir, _ := scoutDir()
			id, _ := identity.LoadOrGenerate(filepath.Join(dir, "identity"))
			trustStore, _ := identity.NewTrustStore(filepath.Join(dir, "trusted"))

			pairingAddr = fmt.Sprintf(":%d", port+1)
			pairingLis, err := net.Listen("tcp", pairingAddr)
			if err != nil {
				return fmt.Errorf("scout: listen pairing on %s: %w", pairingAddr, err)
			}

			pairingGRPC = grpc.NewServer()
			pairingSrv := server.NewPairingServer(id, trustStore)
			pairingSrv.OnPaired = func(deviceID string) {
				scoutServer.NotifyPeerChange()
			}
			pb.RegisterPairingServiceServer(pairingGRPC, pairingSrv)

			go func() {
				_ = pairingGRPC.Serve(pairingLis)
			}()
		}

		printTable(nil)

		// Graceful shutdown
		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			_, _ = fmt.Fprintln(os.Stdout, "\nshutting down gRPC server...")
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
