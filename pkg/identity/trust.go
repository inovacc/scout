package identity

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TrustedDevice holds metadata about a trusted peer.
type TrustedDevice struct {
	DeviceID  string
	ShortID   string
	TrustedAt time.Time
}

// TrustStore manages trusted device certificates on disk.
type TrustStore struct {
	dir string
}

// NewTrustStore creates a TrustStore backed by the given directory.
func NewTrustStore(dir string) (*TrustStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("identity: create trust dir: %w", err)
	}
	return &TrustStore{dir: dir}, nil
}

// Trust adds a device certificate to the trusted set.
func (ts *TrustStore) Trust(deviceID string, certDER []byte) error {
	pemData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	path := filepath.Join(ts.dir, deviceID+".pem")
	return os.WriteFile(path, pemData, 0o644)
}

// IsTrusted checks if a device ID is in the trusted set.
func (ts *TrustStore) IsTrusted(deviceID string) bool {
	path := filepath.Join(ts.dir, deviceID+".pem")
	_, err := os.Stat(path)
	return err == nil
}

// Remove revokes trust for a device.
func (ts *TrustStore) Remove(deviceID string) error {
	path := filepath.Join(ts.dir, deviceID+".pem")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("identity: remove trust: %w", err)
	}
	return nil
}

// List returns all trusted devices.
func (ts *TrustStore) List() ([]TrustedDevice, error) {
	entries, err := os.ReadDir(ts.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("identity: list trusted: %w", err)
	}

	var devices []TrustedDevice
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pem") {
			continue
		}
		deviceID := strings.TrimSuffix(e.Name(), ".pem")
		info, _ := e.Info()
		var trustedAt time.Time
		if info != nil {
			trustedAt = info.ModTime()
		}
		devices = append(devices, TrustedDevice{
			DeviceID:  deviceID,
			ShortID:   ShortID(deviceID),
			TrustedAt: trustedAt,
		})
	}
	return devices, nil
}

// CertPool returns a x509.CertPool containing all trusted device certificates.
func (ts *TrustStore) CertPool() (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	entries, err := os.ReadDir(ts.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return pool, nil
		}
		return nil, fmt.Errorf("identity: read trust dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pem") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ts.dir, e.Name()))
		if err != nil {
			continue
		}
		pool.AppendCertsFromPEM(data)
	}

	return pool, nil
}
