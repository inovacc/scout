//go:build !windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const nativeHostName = "com.inovacc.scout.capture"

// nativeManifestDir returns the Chrome/Chromium NativeMessagingHosts dir for this user.
func nativeManifestDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts"), nil
	default: // linux
		return filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts"), nil
	}
}

func installNativeManifest(extID string) (string, error) {
	dir, err := nativeManifestDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("scout: capture: mkdir manifest dir: %w", err)
	}
	exe, err := exec.LookPath(os.Args[0])
	if err != nil {
		exe, _ = filepath.Abs(os.Args[0])
	}
	manifest := map[string]any{
		"name":            nativeHostName,
		"description":     "Scout Capture native messaging host",
		"path":            exe,
		"type":            "stdio",
		"allowed_origins": []string{"chrome-extension://" + extID + "/"},
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	path := filepath.Join(dir, nativeHostName+".json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", fmt.Errorf("scout: capture: write manifest: %w", err)
	}
	return path, nil
}

func uninstallNativeManifest() error {
	dir, err := nativeManifestDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, nativeHostName+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("scout: capture: remove manifest: %w", err)
	}
	return nil
}
