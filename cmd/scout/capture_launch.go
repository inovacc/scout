package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout/capture"
	"github.com/inovacc/scout/pkg/scout/vault"
)

// vaultFileFor returns a vault.Option pointing to the default vault path under a
// given scout home directory. It ensures the parent profiles/ dir exists so
// vault.Create can write the file (atomicWrite requires the parent to exist).
func vaultFileFor(home string) vault.Option {
	p := filepath.Join(home, "profiles", "vault.bin")
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	return vault.WithPath(p)
}

// runCaptureHostStreams runs the native-messaging host over the given streams.
// It loads the operator-provisioned state (allowed ext-id, public key, spool,
// pairing nonce) and delegates to capture.RunHost. origin is the chrome-extension
// origin Chrome passed as argv; it is cross-checked against the persisted ext-id.
func runCaptureHostStreams(r io.Reader, w io.Writer, origin string) error {
	allowed, err := loadExtID()
	if err != nil {
		return err
	}
	if id, ok := capture.OriginToExtID(origin); ok && id != allowed {
		return fmt.Errorf("scout: capture: launching origin %q does not match installed ext id", origin)
	}
	pubPath, err := capturePubPath()
	if err != nil {
		return err
	}
	pub, err := capture.LoadPub(pubPath)
	if err != nil {
		return err
	}
	spoolDir, err := capture.SpoolDir()
	if err != nil {
		return err
	}
	noncePath, err := captureNoncePath()
	if err != nil {
		return err
	}
	return capture.RunHost(r, w, capture.HostConfig{
		Pub:          pub,
		SpoolDir:     spoolDir,
		AllowedExtID: allowed,
		NoncePath:    noncePath,
	})
}

// maybeRunCaptureHost handles the case where Chrome launched this binary as a
// native-messaging host (argv carries a chrome-extension:// origin). It runs the
// host on os.Stdin/os.Stdout and exits the process. It MUST be called as the very
// first thing in main(), before any bootstrap writes to stdout. Returns without
// acting for a normal CLI invocation.
func maybeRunCaptureHost() {
	origin, ok := capture.IsNativeMessagingLaunch(os.Args[1:])
	if !ok {
		return
	}
	if err := runCaptureHostStreams(os.Stdin, os.Stdout, origin); err != nil {
		// Never print to stdout (the wire); a non-zero exit + stderr is enough.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
