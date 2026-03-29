package main

import "github.com/spf13/cobra"

var grpcCmd = &cobra.Command{
	Use:   "grpc",
	Short: "Control a running gRPC daemon session",
	Long:  "Commands that require a running scout gRPC daemon (scout server). All commands interact with the current session page.",
}
