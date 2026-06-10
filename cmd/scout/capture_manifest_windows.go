//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const nativeHostName = "com.inovacc.scout.capture"

// On Windows the manifest is a JSON file pointed to by a registry key.
func installNativeManifest(extID string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Scout", "NativeMessagingHosts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("scout: capture: mkdir manifest dir: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
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
	k, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Google\Chrome\NativeMessagingHosts\`+nativeHostName, registry.SET_VALUE)
	if err != nil {
		return "", fmt.Errorf("scout: capture: create registry key: %w", err)
	}
	defer func() { _ = k.Close() }()
	if err := k.SetStringValue("", path); err != nil {
		return "", fmt.Errorf("scout: capture: set registry value: %w", err)
	}
	return path, nil
}

func uninstallNativeManifest() error {
	_ = registry.DeleteKey(registry.CURRENT_USER,
		`Software\Google\Chrome\NativeMessagingHosts\`+nativeHostName)
	base, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(base, "Scout", "NativeMessagingHosts", nativeHostName+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("scout: capture: remove manifest: %w", err)
	}
	return nil
}
