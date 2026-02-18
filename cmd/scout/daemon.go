package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/grpc/server"
	"github.com/inovacc/scout/pkg/identity"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// scoutDir returns the path to ~/.scout, creating it if needed.
func scoutDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("scout: home dir: %w", err)
	}

	dir := filepath.Join(home, ".scout")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create config dir: %w", err)
	}

	return dir, nil
}

// pidFilePath returns the path to the daemon PID file.
func pidFilePath() (string, error) {
	dir, err := scoutDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "daemon.pid"), nil
}

// isDaemonReachable checks if a gRPC server is reachable at addr within the given timeout.
func isDaemonReachable(addr string, timeout time.Duration) bool {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return false
	}

	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn.Connect()

	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return true
		}

		if !conn.WaitForStateChange(ctx, state) {
			return false
		}
	}
}

// isLocalAddr returns true if addr refers to localhost or has no host component.
func isLocalAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return true // no port, assume local
	}
	return host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// ensureDaemon checks if a gRPC server is reachable at addr; if not and addr is local, starts one.
func ensureDaemon(addr string) error {
	if isDaemonReachable(addr, 2*time.Second) {
		return nil
	}

	// For remote addresses, also try a raw TCP dial — the server may require mTLS
	// which isDaemonReachable (insecure gRPC) can't establish. A successful TCP
	// connection means the port is open and the client should attempt mTLS.
	if !isLocalAddr(addr) {
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err == nil {
			_ = conn.Close()
			return nil // port is open, let the caller try mTLS
		}
		return fmt.Errorf("scout: server at %s not reachable", addr)
	}

	// Daemon not running — start one
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("scout: find executable: %w", err)
	}

	// Extract port from addr
	port := "50051"
	if parts := strings.SplitN(addr, ":", 2); len(parts) == 2 {
		port = parts[1]
	}

	cmd := exec.Command(exe, "server", "--port", port, "--insecure")
	cmd.Stdout = nil
	cmd.Stderr = nil

	if runtime.GOOS != "windows" {
		// Detach from parent process group on Unix
		setSysProcAttr(cmd)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("scout: start daemon: %w", err)
	}

	// Write PID file
	pidPath, err := pidFilePath()
	if err == nil {
		_ = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	}

	// Release the process so it outlives us
	_ = cmd.Process.Release()

	// Wait for daemon to become available
	for range 20 {
		time.Sleep(250 * time.Millisecond)

		if isDaemonReachable(addr, 1*time.Second) {
			return nil
		}
	}

	return fmt.Errorf("scout: daemon started but not reachable after 5s")
}

// getClientTLS connects using mTLS credentials from the local identity.
func getClientTLS(addr string) (pb.ScoutServiceClient, *grpc.ClientConn, error) {
	dir, err := scoutDir()
	if err != nil {
		return nil, nil, fmt.Errorf("scout: config dir: %w", err)
	}

	id, err := identity.LoadIdentity(filepath.Join(dir, "identity"))
	if err != nil {
		return nil, nil, fmt.Errorf("scout: load identity for TLS: %w", err)
	}

	creds := grpc.WithTransportCredentials(server.ClientTLSCredentials(id))
	return dialClient(addr, creds)
}

// getClient connects to the gRPC daemon using plaintext (insecure).
// Used for local daemon communication. For mTLS connections, use getClientTLS.
func getClient(addr string) (pb.ScoutServiceClient, *grpc.ClientConn, error) {
	return dialClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func dialClient(addr string, creds grpc.DialOption) (pb.ScoutServiceClient, *grpc.ClientConn, error) {

	conn, err := grpc.NewClient(addr,
		creds,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64*1024*1024),
			grpc.MaxCallSendMsgSize(64*1024*1024),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("scout: connect: %w", err)
	}

	return pb.NewScoutServiceClient(conn), conn, nil
}

// resolveClient returns a gRPC client using mTLS or insecure mode based on the --insecure flag.
func resolveClient(cmd *cobra.Command) (pb.ScoutServiceClient, *grpc.ClientConn, error) {
	addr, _ := cmd.Flags().GetString("addr")
	insecureMode, _ := cmd.Flags().GetBool("insecure")

	if insecureMode {
		if err := ensureDaemon(addr); err != nil {
			return nil, nil, err
		}

		return getClient(addr)
	}

	return getClientTLS(addr)
}

// resolveSession returns the active session ID from flag or ~/.scout/current-session.
func resolveSession(sessionFlag string) (string, error) {
	if sessionFlag != "" {
		return sessionFlag, nil
	}

	dir, err := scoutDir()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(filepath.Join(dir, "current-session"))
	if err != nil {
		return "", fmt.Errorf("scout: no active session (use --session or 'scout session use <id>')")
	}

	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", fmt.Errorf("scout: no active session (use --session or 'scout session use <id>')")
	}

	return id, nil
}

// saveCurrentSession stores the session ID in ~/.scout/current-session.
func saveCurrentSession(id string) error {
	dir, err := scoutDir()
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "current-session"), []byte(id), 0o644)
}
