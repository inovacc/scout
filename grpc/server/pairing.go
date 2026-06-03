package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base32"
	"fmt"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	identity2 "github.com/inovacc/scout/pkg/scout/identity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// PairingTokenMetadataKey is the gRPC metadata key that carries the
// out-of-band pairing token from client to server.
const PairingTokenMetadataKey = "x-pairing-token"

// PairingServer implements the PairingService gRPC service.
// It runs on an insecure listener to allow certificate exchange
// between devices that want to establish mTLS trust.
//
// Pairing requires an out-of-band token: the server generates (or is
// configured with) a secret token that the operator must convey to the
// client over a trusted channel. Without a matching token the Pair RPC
// is rejected and the peer is NOT enrolled in the trust store. This
// closes the trust-on-first-use auto-enroll where any host that could
// reach the pairing port gained full remote browser control.
type PairingServer struct {
	pb.UnimplementedPairingServiceServer

	id         *identity2.Identity
	trustStore *identity2.TrustStore

	// pairingToken is the required out-of-band secret. Must be non-empty.
	pairingToken string

	// OnPaired is called after a successful pairing. Can be nil.
	OnPaired func(deviceID string)
}

// GeneratePairingToken returns a cryptographically random pairing token
// suitable for out-of-band exchange. It is base32-encoded (no padding,
// uppercase) so it is easy to read aloud or type.
func GeneratePairingToken() (string, error) {
	buf := make([]byte, 20) // 160 bits
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("scout: pair: generate token: %w", err)
	}

	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// NewPairingServer creates a new PairingServer that requires the given
// out-of-band pairing token. The token must be presented by clients in
// the "x-pairing-token" gRPC metadata header; mismatches are rejected
// with codes.PermissionDenied before any trust enrollment occurs.
func NewPairingServer(id *identity2.Identity, ts *identity2.TrustStore, pairingToken string) *PairingServer {
	return &PairingServer{
		id:           id,
		trustStore:   ts,
		pairingToken: pairingToken,
	}
}

// tokenFromContext extracts the client-presented pairing token from the
// incoming gRPC metadata, if any.
func tokenFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	vals := md.Get(PairingTokenMetadataKey)
	if len(vals) == 0 {
		return ""
	}

	return vals[0]
}

// Pair handles a pairing request from a client. It verifies the client's
// certificate matches the claimed device ID, stores the client cert in the
// trust store, and returns the server's device ID and certificate.
func (s *PairingServer) Pair(ctx context.Context, req *pb.PairRequest) (*pb.PairResponse, error) {
	// Require a matching out-of-band pairing token BEFORE any work or
	// trust enrollment. A misconfigured server (empty token) rejects all
	// pairing rather than failing open.
	if s.pairingToken == "" {
		return nil, status.Error(codes.PermissionDenied, "scout: pair: pairing disabled (no token configured)")
	}

	presented := tokenFromContext(ctx)
	if subtle.ConstantTimeCompare([]byte(presented), []byte(s.pairingToken)) != 1 {
		return nil, status.Error(codes.PermissionDenied, "scout: pair: invalid or missing pairing token")
	}

	if len(req.GetCertDer()) == 0 {
		return nil, fmt.Errorf("scout: pair: empty certificate")
	}

	if req.GetDeviceId() == "" {
		return nil, fmt.Errorf("scout: pair: empty device ID")
	}

	// Parse and verify the client certificate matches the claimed device ID.
	cert, err := x509.ParseCertificate(req.GetCertDer())
	if err != nil {
		return nil, fmt.Errorf("scout: pair: parse client cert: %w", err)
	}

	derivedID, err := identity2.DeviceIDFromCert(cert)
	if err != nil {
		return nil, fmt.Errorf("scout: pair: derive device ID: %w", err)
	}
	if derivedID != req.GetDeviceId() {
		return nil, fmt.Errorf("scout: pair: device ID mismatch (claimed %s, derived %s)",
			identity2.ShortID(req.GetDeviceId()), identity2.ShortID(derivedID))
	}

	// Store the client's certificate in our trust store.
	if err := s.trustStore.Trust(derivedID, req.GetCertDer()); err != nil {
		return nil, fmt.Errorf("scout: pair: store client cert: %w", err)
	}

	if s.OnPaired != nil {
		s.OnPaired(derivedID)
	}

	// Return our device ID and certificate.
	return &pb.PairResponse{
		ServerDeviceId: s.id.DeviceID,
		ServerCertDer:  s.id.Certificate.Certificate[0],
	}, nil
}
