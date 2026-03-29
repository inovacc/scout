package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"runtime/debug"

	identity2 "github.com/inovacc/scout/pkg/scout/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

// NewTLSServer creates a gRPC server with mTLS authentication using the given identity and trust store.
// Peers must present a certificate whose device ID is in the trust store.
func NewTLSServer(id *identity2.Identity, trustStore *identity2.TrustStore, opts ...grpc.ServerOption) (*grpc.Server, *ScoutServer, error) {
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{id.Certificate},
		ClientAuth:   tls.RequireAnyClientCert,
		MinVersion:   tls.VersionTLS13,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("no client certificate provided")
			}

			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("parse client cert: %w", err)
			}

			deviceID, err := identity2.DeviceIDFromCert(cert)
			if err != nil {
				return fmt.Errorf("compute device ID: %w", err)
			}
			if !trustStore.IsTrusted(deviceID) {
				return fmt.Errorf("device %s not trusted", identity2.ShortID(deviceID))
			}

			return nil
		},
	}

	creds := credentials.NewTLS(tlsCfg)
	allOpts := append([]grpc.ServerOption{
		grpc.Creds(creds),
		grpc.UnaryInterceptor(panicRecoveryUnary()),
		grpc.StreamInterceptor(panicRecoveryStream()),
	}, opts...)
	grpcServer := grpc.NewServer(allOpts...)

	scoutServer := New()

	return grpcServer, scoutServer, nil
}

// ClientTLSCredentials creates gRPC transport credentials for a client using mTLS.
func ClientTLSCredentials(id *identity2.Identity) credentials.TransportCredentials {
	tlsCfg := &tls.Config{
		Certificates:       []tls.Certificate{id.Certificate},
		InsecureSkipVerify: true, //nolint:gosec // self-signed certs; we verify via device ID
		MinVersion:         tls.VersionTLS13,
	}

	return credentials.NewTLS(tlsCfg)
}

// panicRecoveryUnary returns a gRPC unary interceptor that recovers from panics
// in rod-inherited code, logging the stack trace and returning an Internal error.
func panicRecoveryUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("scout: grpc: recovered panic", "method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
				err = status.Errorf(codes.Internal, "scout: grpc: internal error: %v", r)
			}
		}()
		return handler(ctx, req)
	}
}

// panicRecoveryStream returns a gRPC stream interceptor that recovers from panics.
func panicRecoveryStream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("scout: grpc: recovered panic in stream", "method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
				err = status.Errorf(codes.Internal, "scout: grpc: internal error: %v", r)
			}
		}()
		return handler(srv, ss)
	}
}
