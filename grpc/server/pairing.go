package server

import (
	"context"
	"crypto/x509"
	"fmt"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/pkg/identity"
)

// PairingServer implements the PairingService gRPC service.
// It runs on an insecure listener to allow certificate exchange
// between devices that want to establish mTLS trust.
type PairingServer struct {
	pb.UnimplementedPairingServiceServer

	id         *identity.Identity
	trustStore *identity.TrustStore

	// OnPaired is called after a successful pairing. Can be nil.
	OnPaired func(deviceID string)
}

// NewPairingServer creates a new PairingServer.
func NewPairingServer(id *identity.Identity, ts *identity.TrustStore) *PairingServer {
	return &PairingServer{
		id:         id,
		trustStore: ts,
	}
}

// Pair handles a pairing request from a client. It verifies the client's
// certificate matches the claimed device ID, stores the client cert in the
// trust store, and returns the server's device ID and certificate.
func (s *PairingServer) Pair(_ context.Context, req *pb.PairRequest) (*pb.PairResponse, error) {
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

	derivedID := identity.DeviceIDFromCert(cert)
	if derivedID != req.GetDeviceId() {
		return nil, fmt.Errorf("scout: pair: device ID mismatch (claimed %s, derived %s)",
			identity.ShortID(req.GetDeviceId()), identity.ShortID(derivedID))
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
