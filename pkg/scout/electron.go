package scout

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/inovacc/scout/pkg/scout/rod/lib/launcher"
	"github.com/inovacc/scout/pkg/scout/rod/lib/launcher/flags"
)

// launchElectron starts an Electron app with CDP debugging enabled
// and returns the CDP WebSocket URL plus the launcher for cleanup.
func launchElectron(o *options) (string, *launcher.Launcher, error) {
	binPath, err := resolveElectron(context.Background(), o.electronVersion)
	if err != nil {
		return "", nil, fmt.Errorf("scout: resolve electron: %w", err)
	}

	l := launcher.New().
		Bin(binPath).
		RemoteDebuggingPort(0) // random port

	if o.noSandbox {
		l = l.NoSandbox(true)
	}

	if o.userDataDir != "" {
		l = l.UserDataDir(o.userDataDir)
	}

	if len(o.env) > 0 {
		l = l.Env(o.env...)
	}

	// Apply custom launch flags.
	for name, values := range o.launchFlags {
		l = l.Set(flags.Flag(name), values...)
	}

	// Electron apps don't use headless mode the same way Chrome does.
	// Remove headless flag — Electron doesn't support it.
	l = l.Delete(flags.Headless)

	// Set app path as positional argument.
	if o.electronApp != "" {
		l = l.Set(flags.Arguments, o.electronApp)
	}

	u, err := l.Launch()
	if err != nil {
		return "", nil, fmt.Errorf("scout: launch electron: %w", err)
	}

	return u, l, nil
}

// resolveElectron finds or downloads the Electron binary.
// If version is set, downloads that specific version.
// Otherwise checks PATH, then downloads the latest version.
func resolveElectron(ctx context.Context, version string) (string, error) {
	if version != "" {
		return DownloadElectron(ctx, version)
	}

	// Check PATH for electron binary.
	if path, err := exec.LookPath("electron"); err == nil {
		return path, nil
	}

	// Download latest.
	return DownloadLatestElectron(ctx)
}

// lookupElectronCDP checks if an Electron app is already running with CDP enabled
// and returns the WebSocket URL. This is used by WithElectronCDP.
func lookupElectronCDP(endpoint string) (string, error) {
	if strings.Contains(endpoint, "/devtools/") {
		return endpoint, nil
	}

	resolved, err := launcher.ResolveURL(endpoint)
	if err != nil {
		return endpoint, nil
	}

	return resolved, nil
}
