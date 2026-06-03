package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"

	identity2 "github.com/inovacc/scout/pkg/scout/identity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// ctxForDevice returns a context carrying an mTLS peer whose client certificate
// belongs to the given identity, mirroring what gRPC populates for a real
// authenticated call.
func ctxForDevice(t *testing.T, id *identity2.Identity) context.Context {
	t.Helper()

	leaf, err := x509.ParseCertificate(id.Certificate.Certificate[0])
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	return peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}},
		},
	})
}

func TestCallerDeviceID(t *testing.T) {
	dev, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	if got := callerDeviceID(ctxForDevice(t, dev)); got != dev.DeviceID {
		t.Errorf("callerDeviceID(mTLS) = %q; want %q", got, dev.DeviceID)
	}

	if got := callerDeviceID(context.Background()); got != unknownDeviceID {
		t.Errorf("callerDeviceID(no peer) = %q; want %q", got, unknownDeviceID)
	}
}

// A session created by device A must not be reachable by another trusted
// device B — getSession is the single chokepoint all 31 session RPCs route
// through, so enforcing ownership here protects every one of them.
func TestGetSession_OwnershipEnforcement(t *testing.T) {
	devA, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	devB, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	s := &ScoutServer{}

	const sid = "session-owned-by-A"

	s.sessions.Store(sid, &session{})
	s.sessionPeer.Store(sid, devA.DeviceID)

	// Owner device A is allowed.
	if _, err := s.getSession(ctxForDevice(t, devA), sid); err != nil {
		t.Errorf("owner device A was denied: %v", err)
	}

	// Non-owner device B is rejected with PermissionDenied.
	if _, err := s.getSession(ctxForDevice(t, devB), sid); status.Code(err) != codes.PermissionDenied {
		t.Errorf("device B got %v; want PermissionDenied", err)
	}

	// Insecure / loopback caller (no client cert) bypasses enforcement so the
	// local daemon keeps working.
	if _, err := s.getSession(context.Background(), sid); err != nil {
		t.Errorf("insecure caller was denied: %v", err)
	}

	// Unknown session id returns NotFound regardless of caller identity.
	if _, err := s.getSession(ctxForDevice(t, devB), "no-such-session"); status.Code(err) != codes.NotFound {
		t.Errorf("unknown id got %v; want NotFound", err)
	}
}

// A session whose owner was never bound to a real identity (created over the
// insecure/loopback transport, recorded as unknownDeviceID) stays accessible —
// there is no identity to enforce, and forcing one would break the local daemon.
func TestGetSession_UnknownOwnerNotEnforced(t *testing.T) {
	devB, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	s := &ScoutServer{}

	const sid = "session-from-insecure-mode"

	s.sessions.Store(sid, &session{})
	s.sessionPeer.Store(sid, unknownDeviceID)

	if _, err := s.getSession(ctxForDevice(t, devB), sid); err != nil {
		t.Errorf("unknown-owner session should remain accessible, got: %v", err)
	}
}
