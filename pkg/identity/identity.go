// Package identity provides Syncthing-style device identity for mTLS authentication.
// Each scout instance has an ECDSA P-256 keypair and self-signed certificate.
// Device IDs are derived from the SHA-256 hash of the certificate's DER encoding,
// formatted as base32 with Luhn check digits.
package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base32"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Identity holds a TLS certificate and private key for a scout instance.
type Identity struct {
	Certificate tls.Certificate
	DeviceID    string
}

// GenerateIdentity creates a new ECDSA P-256 keypair and self-signed certificate.
func GenerateIdentity() (*Identity, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("identity: generate key: %w", err)
	}

	serialMax := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialMax)
	if err != nil {
		return nil, fmt.Errorf("identity: generate serial: %w", err)
	}

	notBefore := time.Now().Truncate(24 * time.Hour)
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour)

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "scout",
			Organization: []string{"Scout"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("identity: create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("identity: marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("identity: parse keypair: %w", err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, fmt.Errorf("identity: parse cert: %w", err)
	}

	return &Identity{
		Certificate: tlsCert,
		DeviceID:    DeviceIDFromCert(cert),
	}, nil
}

// DeviceIDFromCert computes the device ID from a certificate's DER encoding.
// Format: SHA-256 → base32 (52 chars) → add Luhn check digits (56 chars) → chunk as 8×7 with dashes.
func DeviceIDFromCert(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	encoded = strings.TrimRight(encoded, "=")

	withLuhn, err := luhnify(encoded)
	if err != nil {
		panic(fmt.Sprintf("identity: luhnify: %v", err))
	}

	return chunkify(withLuhn)
}

// ValidateDeviceID checks whether a device ID string has valid format and Luhn check digits.
func ValidateDeviceID(id string) bool {
	raw := unchunkify(strings.ToUpper(id))
	if len(raw) != 56 {
		return false
	}
	_, err := unluhnify(raw)
	return err == nil
}

// ShortID returns the first 7 characters of a device ID for display.
func ShortID(id string) string {
	clean := unchunkify(id)
	if len(clean) >= 7 {
		return clean[:7]
	}
	return clean
}

// SaveIdentity writes the certificate and private key to PEM files in the given directory.
func SaveIdentity(id *Identity, dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("identity: create dir: %w", err)
	}

	// Extract cert PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: id.Certificate.Certificate[0],
	})
	if err := os.WriteFile(filepath.Join(dir, "cert.pem"), certPEM, 0o644); err != nil {
		return fmt.Errorf("identity: write cert: %w", err)
	}

	// Extract key PEM
	keyBytes, err := x509.MarshalECPrivateKey(id.Certificate.PrivateKey.(*ecdsa.PrivateKey))
	if err != nil {
		return fmt.Errorf("identity: marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	if err := os.WriteFile(filepath.Join(dir, "key.pem"), keyPEM, 0o600); err != nil {
		return fmt.Errorf("identity: write key: %w", err)
	}

	return nil
}

// LoadIdentity reads a certificate and private key from PEM files in the given directory.
func LoadIdentity(dir string) (*Identity, error) {
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("identity: load keypair: %w", err)
	}

	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("identity: parse cert: %w", err)
	}

	return &Identity{
		Certificate: tlsCert,
		DeviceID:    DeviceIDFromCert(cert),
	}, nil
}

// LoadOrGenerate loads an identity from dir, or generates and saves a new one if none exists.
func LoadOrGenerate(dir string) (*Identity, error) {
	id, err := LoadIdentity(dir)
	if err == nil {
		return id, nil
	}

	id, err = GenerateIdentity()
	if err != nil {
		return nil, err
	}

	if err := SaveIdentity(id, dir); err != nil {
		return nil, err
	}

	return id, nil
}
