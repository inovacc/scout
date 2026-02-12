package cli

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/grpc/server"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().Int("port", 50051, "gRPC server port")
	serverCmd.Flags().Bool("reflection", true, "enable gRPC reflection")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the gRPC browser automation server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		port, _ := cmd.Flags().GetInt("port")
		enableReflection, _ := cmd.Flags().GetBool("reflection")

		addr := fmt.Sprintf(":%d", port)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("scout: listen on %s: %w", addr, err)
		}

		grpcServer := grpc.NewServer(
			grpc.MaxRecvMsgSize(64*1024*1024),
			grpc.MaxSendMsgSize(64*1024*1024),
		)

		scoutServer := server.New()
		pb.RegisterScoutServiceServer(grpcServer, scoutServer)

		if enableReflection {
			reflection.Register(grpcServer)
		}

		// Graceful shutdown
		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			log.Println("shutting down gRPC server...")
			grpcServer.GracefulStop()
		}()

		log.Printf("scout gRPC server listening on %s", addr)
		if err := grpcServer.Serve(lis); err != nil {
			return fmt.Errorf("scout: serve: %w", err)
		}

		return nil
	},
}
