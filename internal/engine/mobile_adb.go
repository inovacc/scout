package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ADBDevice represents a connected Android device.
type ADBDevice struct {
	Serial string `json:"serial"`
	Model  string `json:"model"`
	State  string `json:"state"` // "device", "offline", "unauthorized"
}

// ListADBDevices returns connected Android devices.
func ListADBDevices(ctx context.Context, adbPath string) ([]ADBDevice, error) {
	if adbPath == "" {
		adbPath = "adb"
	}

	out, err := exec.CommandContext(ctx, adbPath, "devices", "-l").Output()
	if err != nil {
		return nil, fmt.Errorf("scout: adb devices: %w", err)
	}

	var devices []ADBDevice
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of") || strings.HasPrefix(line, "*") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		dev := ADBDevice{
			Serial: parts[0],
			State:  parts[1],
		}

		// Parse model from "model:Pixel_6" suffix
		for _, p := range parts[2:] {
			if after, ok := strings.CutPrefix(p, "model:"); ok {
				dev.Model = after
			}
		}

		devices = append(devices, dev)
	}

	return devices, nil
}

// SetupADBForward creates a TCP port forward from localhost to the device's Chrome CDP socket.
func SetupADBForward(ctx context.Context, cfg MobileConfig) (string, error) {
	adb := cfg.ADBPath
	if adb == "" {
		adb = "adb"
	}

	port := cfg.CDPPort
	if port == 0 {
		port = 9222
	}

	args := []string{}
	if cfg.DeviceID != "" {
		args = append(args, "-s", cfg.DeviceID)
	}

	// Forward local port to Chrome's devtools socket
	socket := "localabstract:chrome_devtools_remote"
	args = append(args, "forward", fmt.Sprintf("tcp:%d", port), socket)

	cmd := exec.CommandContext(ctx, adb, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("scout: adb forward: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Wait briefly for the forward to establish
	time.Sleep(200 * time.Millisecond)

	return fmt.Sprintf("http://127.0.0.1:%d", port), nil
}

// RemoveADBForward removes a previously created port forward.
func RemoveADBForward(ctx context.Context, cfg MobileConfig) error {
	adb := cfg.ADBPath
	if adb == "" {
		adb = "adb"
	}

	port := cfg.CDPPort
	if port == 0 {
		port = 9222
	}

	args := []string{}
	if cfg.DeviceID != "" {
		args = append(args, "-s", cfg.DeviceID)
	}

	args = append(args, "forward", "--remove", fmt.Sprintf("tcp:%d", port))

	cmd := exec.CommandContext(ctx, adb, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scout: adb forward remove: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}
