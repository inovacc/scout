package scout

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExtensionInfo holds metadata about a locally stored Chrome extension.
type ExtensionInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// crxUpdateURL is the Chrome Web Store URL template for downloading CRX files.
const crxUpdateURL = "https://clients2.google.com/service/update2/crx?response=redirect&prodversion=131.0&acceptformat=crx2,crx3&x=id%%3D%s%%26installsource%%3Dondemand%%26uc"

// crxDownloadTimeout is the HTTP timeout for downloading CRX files.
const crxDownloadTimeout = 60 * time.Second

// DownloadExtension downloads a Chrome extension by ID from the Chrome Web Store,
// unpacks the CRX3 file, and stores it in ~/.scout/extensions/<id>/.
func DownloadExtension(id string) (*ExtensionInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("scout: extension id is empty")
	}

	data, err := downloadCRX(id)
	if err != nil {
		return nil, err
	}

	extDir, err := ExtensionDir()
	if err != nil {
		return nil, err
	}

	destDir := filepath.Join(extDir, id)

	// Remove existing if present, to get a clean install.
	if err := os.RemoveAll(destDir); err != nil {
		return nil, fmt.Errorf("scout: clean extension dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("scout: create extension dir: %w", err)
	}

	if err := unpackCRX(data, destDir); err != nil {
		return nil, err
	}

	name, version, err := readManifest(destDir)
	if err != nil {
		return nil, err
	}

	return &ExtensionInfo{
		ID:      id,
		Name:    name,
		Version: version,
		Path:    destDir,
	}, nil
}

// ListLocalExtensions returns metadata for all extensions stored in ~/.scout/extensions/.
func ListLocalExtensions() ([]ExtensionInfo, error) {
	extDir, err := ExtensionDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(extDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scout: read extensions dir: %w", err)
	}

	var exts []ExtensionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		id := entry.Name()
		dir := filepath.Join(extDir, id)

		name, version, err := readManifest(dir)
		if err != nil {
			// Skip directories without a valid manifest.
			continue
		}

		exts = append(exts, ExtensionInfo{
			ID:      id,
			Name:    name,
			Version: version,
			Path:    dir,
		})
	}

	return exts, nil
}

// RemoveExtension deletes a locally stored extension by ID.
func RemoveExtension(id string) error {
	if id == "" {
		return fmt.Errorf("scout: extension id is empty")
	}

	extDir, err := ExtensionDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(extDir, id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("scout: extension %q not found", id)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("scout: remove extension: %w", err)
	}

	return nil
}

// ExtensionDir returns the path to ~/.scout/extensions/, creating it if needed.
func ExtensionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("scout: user home dir: %w", err)
	}

	dir := filepath.Join(home, ".scout", "extensions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create extensions dir: %w", err)
	}

	return dir, nil
}

// extensionPathByID returns the local path for a downloaded extension.
// Returns an error if the extension has not been downloaded.
func extensionPathByID(id string) (string, error) {
	extDir, err := ExtensionDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(extDir, id)
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		return "", fmt.Errorf("scout: extension %q not downloaded; run DownloadExtension first", id)
	}

	return dir, nil
}

// downloadCRX fetches a CRX file from the Chrome Web Store.
func downloadCRX(id string) ([]byte, error) {
	url := fmt.Sprintf(crxUpdateURL, id)

	client := &http.Client{Timeout: crxDownloadTimeout}

	resp, err := client.Get(url) //nolint:gosec,noctx // URL is constructed from a known template
	if err != nil {
		return nil, fmt.Errorf("scout: download extension: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scout: download extension: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("scout: read extension data: %w", err)
	}

	return data, nil
}

// unpackCRX validates and extracts a CRX2 or CRX3 file into destDir.
//
// CRX3: "Cr24" (4B) + version=3 uint32 LE (4B) + header_length uint32 LE (4B) + protobuf header + ZIP.
// CRX2: "Cr24" (4B) + version=2 uint32 LE (4B) + pubkey_length uint32 LE (4B) + sig_length uint32 LE (4B) + pubkey + sig + ZIP.
func unpackCRX(data []byte, destDir string) error {
	if len(data) < 12 {
		return fmt.Errorf("scout: CRX data too short")
	}

	magic := string(data[:4])
	if magic != "Cr24" {
		return fmt.Errorf("scout: invalid CRX magic: %q", magic)
	}

	version := binary.LittleEndian.Uint32(data[4:8])

	var zipStart int
	switch version {
	case 3:
		headerLen := binary.LittleEndian.Uint32(data[8:12])
		zipStart = 12 + int(headerLen)
	case 2:
		if len(data) < 16 {
			return fmt.Errorf("scout: CRX2 data too short")
		}
		pubKeyLen := binary.LittleEndian.Uint32(data[8:12])
		sigLen := binary.LittleEndian.Uint32(data[12:16])
		zipStart = 16 + int(pubKeyLen) + int(sigLen)
	default:
		return fmt.Errorf("scout: unsupported CRX version: %d", version)
	}

	if zipStart > len(data) {
		return fmt.Errorf("scout: CRX header length exceeds data size")
	}

	zipData := data[zipStart:]

	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("scout: open CRX zip: %w", err)
	}

	for _, f := range zr.File {
		target := filepath.Join(destDir, f.Name) //nolint:gosec // trusted archive from Chrome Web Store

		// Ensure the target stays within destDir (zip slip protection).
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("scout: zip slip detected: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("scout: create dir %s: %w", f.Name, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("scout: create parent dir: %w", err)
		}

		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}

	return nil
}

func extractZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("scout: open zip entry %s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("scout: create file %s: %w", f.Name, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("scout: write file %s: %w", f.Name, err)
	}

	return nil
}

// readManifest reads name and version from a Chrome extension's manifest.json.
func readManifest(dir string) (name, version string, err error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return "", "", fmt.Errorf("scout: read manifest.json: %w", err)
	}

	var manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", "", fmt.Errorf("scout: parse manifest.json: %w", err)
	}

	name = manifest.Name
	if name == "" {
		name = "unknown"
	}

	version = manifest.Version
	if version == "" {
		version = "0.0.0"
	}

	return name, version, nil
}
