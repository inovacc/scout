package server

import (
	"testing"

	"github.com/inovacc/scout/pkg/identity"
)

func TestNewTLSServer(t *testing.T) {
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}

	dir := t.TempDir()
	ts, err := identity.NewTrustStore(dir)
	if err != nil {
		t.Fatalf("NewTrustStore: %v", err)
	}

	grpcSrv, scoutSrv, err := NewTLSServer(id, ts)
	if err != nil {
		t.Fatalf("NewTLSServer: %v", err)
	}

	if grpcSrv == nil {
		t.Error("grpcServer should not be nil")
	}
	if scoutSrv == nil {
		t.Error("scoutServer should not be nil")
	}

	grpcSrv.Stop()
}

func TestClientTLSCredentials(t *testing.T) {
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}

	creds := ClientTLSCredentials(id)
	if creds == nil {
		t.Error("credentials should not be nil")
	}

	info := creds.Info()
	if info.SecurityProtocol != "tls" {
		t.Errorf("protocol = %q, want tls", info.SecurityProtocol)
	}
}
