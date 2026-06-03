package server

import (
	"context"
	"strings"
	"testing"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	identity2 "github.com/inovacc/scout/pkg/scout/identity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testPairingToken = "TEST-PAIRING-TOKEN"

// ctxWithToken returns a context carrying the given pairing token in
// incoming gRPC metadata, mimicking what the transport delivers.
func ctxWithToken(token string) context.Context {
	md := metadata.Pairs(PairingTokenMetadataKey, token)
	return metadata.NewIncomingContext(context.Background(), md)
}

func newTestPairingServer(t *testing.T) (*PairingServer, *identity2.Identity) { //nolint:unparam
	t.Helper()

	serverID, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity server: %v", err)
	}

	dir := t.TempDir()

	ts, err := identity2.NewTrustStore(dir)
	if err != nil {
		t.Fatalf("NewTrustStore: %v", err)
	}

	ps := NewPairingServer(serverID, ts, testPairingToken)

	return ps, serverID
}

func TestPair_HappyPath(t *testing.T) {
	ps, _ := newTestPairingServer(t)

	clientID, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity client: %v", err)
	}

	var pairedDeviceID string

	ps.OnPaired = func(deviceID string) {
		pairedDeviceID = deviceID
	}

	resp, err := ps.Pair(ctxWithToken(testPairingToken), &pb.PairRequest{
		DeviceId: clientID.DeviceID,
		CertDer:  clientID.Certificate.Certificate[0],
	})
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}

	// The client should be enrolled in the trust store on success.
	if !ps.trustStore.IsTrusted(clientID.DeviceID) {
		t.Errorf("client %s should be trusted after successful pairing", clientID.DeviceID)
	}

	if resp.GetServerDeviceId() == "" {
		t.Error("server device ID should not be empty")
	}

	if len(resp.GetServerCertDer()) == 0 {
		t.Error("server cert should not be empty")
	}

	if pairedDeviceID != clientID.DeviceID {
		t.Errorf("OnPaired called with %q, want %q", pairedDeviceID, clientID.DeviceID)
	}
}

func TestPair_EmptyCert(t *testing.T) {
	ps, _ := newTestPairingServer(t)

	_, err := ps.Pair(ctxWithToken(testPairingToken), &pb.PairRequest{
		DeviceId: "some-device-id",
		CertDer:  nil,
	})
	if err == nil {
		t.Fatal("expected error for empty cert")
	}

	if !strings.Contains(err.Error(), "empty certificate") {
		t.Errorf("error = %q, want contains 'empty certificate'", err.Error())
	}
}

func TestPair_EmptyDeviceID(t *testing.T) {
	ps, _ := newTestPairingServer(t)

	clientID, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}

	_, err = ps.Pair(ctxWithToken(testPairingToken), &pb.PairRequest{
		DeviceId: "",
		CertDer:  clientID.Certificate.Certificate[0],
	})
	if err == nil {
		t.Fatal("expected error for empty device ID")
	}

	if !strings.Contains(err.Error(), "empty device ID") {
		t.Errorf("error = %q, want contains 'empty device ID'", err.Error())
	}
}

func TestPair_CertParseFail(t *testing.T) {
	ps, _ := newTestPairingServer(t)

	_, err := ps.Pair(ctxWithToken(testPairingToken), &pb.PairRequest{
		DeviceId: "some-device-id",
		CertDer:  []byte("not a valid cert"),
	})
	if err == nil {
		t.Fatal("expected error for invalid cert")
	}

	if !strings.Contains(err.Error(), "parse client cert") {
		t.Errorf("error = %q, want contains 'parse client cert'", err.Error())
	}
}

func TestPair_DeviceIDMismatch(t *testing.T) {
	ps, _ := newTestPairingServer(t)

	clientID, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}

	_, err = ps.Pair(ctxWithToken(testPairingToken), &pb.PairRequest{
		DeviceId: "WRONG-DEVICE-ID",
		CertDer:  clientID.Certificate.Certificate[0],
	})
	if err == nil {
		t.Fatal("expected error for device ID mismatch")
	}

	if !strings.Contains(err.Error(), "device ID mismatch") {
		t.Errorf("error = %q, want contains 'device ID mismatch'", err.Error())
	}
}

// TestPair_NoToken verifies an unauthorized peer presenting no pairing
// token is rejected with PermissionDenied and NOT enrolled in trust.
func TestPair_NoToken(t *testing.T) {
	ps, _ := newTestPairingServer(t)

	clientID, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity client: %v", err)
	}

	_, err = ps.Pair(context.Background(), &pb.PairRequest{
		DeviceId: clientID.DeviceID,
		CertDer:  clientID.Certificate.Certificate[0],
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}

	if ps.trustStore.IsTrusted(clientID.DeviceID) {
		t.Errorf("client %s must NOT be trusted after rejected pairing", clientID.DeviceID)
	}
}

// TestPair_WrongToken verifies a peer presenting an incorrect token is
// rejected with PermissionDenied and NOT enrolled in trust.
func TestPair_WrongToken(t *testing.T) {
	ps, _ := newTestPairingServer(t)

	clientID, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity client: %v", err)
	}

	_, err = ps.Pair(ctxWithToken("WRONG-TOKEN"), &pb.PairRequest{
		DeviceId: clientID.DeviceID,
		CertDer:  clientID.Certificate.Certificate[0],
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}

	if ps.trustStore.IsTrusted(clientID.DeviceID) {
		t.Errorf("client %s must NOT be trusted after rejected pairing", clientID.DeviceID)
	}
}

// TestPair_EmptyServerToken verifies a server configured with no token
// rejects all pairing (fails closed) and never enrolls a peer.
func TestPair_EmptyServerToken(t *testing.T) {
	serverID, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity server: %v", err)
	}

	ts, err := identity2.NewTrustStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrustStore: %v", err)
	}

	ps := NewPairingServer(serverID, ts, "")

	clientID, err := identity2.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity client: %v", err)
	}

	_, err = ps.Pair(ctxWithToken("anything"), &pb.PairRequest{
		DeviceId: clientID.DeviceID,
		CertDer:  clientID.Certificate.Certificate[0],
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}

	if ts.IsTrusted(clientID.DeviceID) {
		t.Errorf("client %s must NOT be trusted when server has no token", clientID.DeviceID)
	}
}

// TestGeneratePairingToken verifies generated tokens are non-empty and unique.
func TestGeneratePairingToken(t *testing.T) {
	a, err := GeneratePairingToken()
	if err != nil {
		t.Fatalf("GeneratePairingToken: %v", err)
	}

	b, err := GeneratePairingToken()
	if err != nil {
		t.Fatalf("GeneratePairingToken: %v", err)
	}

	if a == "" || b == "" {
		t.Fatal("generated token must not be empty")
	}

	if a == b {
		t.Error("generated tokens should be unique")
	}
}
