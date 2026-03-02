package server

import (
	"context"
	"strings"
	"testing"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	identity2 "github.com/inovacc/scout/pkg/scout/identity"
)

func newTestPairingServer(t *testing.T) (*PairingServer, *identity2.Identity) {
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

	ps := NewPairingServer(serverID, ts)

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

	resp, err := ps.Pair(context.Background(), &pb.PairRequest{
		DeviceId: clientID.DeviceID,
		CertDer:  clientID.Certificate.Certificate[0],
	})
	if err != nil {
		t.Fatalf("Pair: %v", err)
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

	_, err := ps.Pair(context.Background(), &pb.PairRequest{
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

	_, err = ps.Pair(context.Background(), &pb.PairRequest{
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

	_, err := ps.Pair(context.Background(), &pb.PairRequest{
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

	_, err = ps.Pair(context.Background(), &pb.PairRequest{
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
