package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/inovacc/scout/internal/engine/scouthome"
	"github.com/inovacc/scout/pkg/scout/capture"
	"github.com/inovacc/scout/pkg/scout/vault"
)

func capturePubPath() (string, error) {
	base, err := scouthome.Sub("captures")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "capture.pub"), nil
}

func captureNoncePath() (string, error) {
	base, err := scouthome.Sub("captures")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pairing.nonce"), nil
}

// generateExtensionKey creates an RSA-2048 keypair for the extension, writes the
// PKCS#8 private key PEM (0600) into dir, and returns the manifest.json "key"
// value (base64 DER SPKI) plus the derived stable extension ID.
func generateExtensionKey(dir string) (keyValue, extID string, err error) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("scout: capture: generate extension key: %w", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("scout: capture: marshal public key: %w", err)
	}
	priv, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return "", "", fmt.Errorf("scout: capture: marshal private key: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", fmt.Errorf("scout: capture: mkdir key dir: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv})
	if err := os.WriteFile(filepath.Join(dir, "extension_key.pem"), pemBytes, 0o600); err != nil {
		return "", "", fmt.Errorf("scout: capture: write extension key: %w", err)
	}
	return capture.ManifestKey(der), capture.ExtensionID(der), nil
}

var captureHostKeygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate a pinned extension keypair; print the manifest key + stable extension ID",
	RunE: func(cmd *cobra.Command, _ []string) error {
		base, err := scouthome.Sub("captures")
		if err != nil {
			return err
		}
		keyValue, extID, err := generateExtensionKey(base)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"extension id: %s\n\nAdd this to extensions/scout-capture/manifest.json:\n  \"key\": \"%s\"\n\nThen run: scout capture-host install %s\n",
			extID, keyValue, extID)
		return nil
	},
}

var vaultCaptureKeyCmd = &cobra.Command{
	Use:   "capture-key",
	Short: "Manage the Scout Capture keypair (sub: init)",
}

var vaultCaptureKeyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the X25519 capture keypair (private key stored in the vault)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		rotate, _ := cmd.Flags().GetBool("rotate")
		pass, err := readPassphraseBytes(cmd.ErrOrStderr(), "Vault passphrase: ")
		if err != nil {
			return err
		}
		defer zeroBytesCLI(pass)
		v, err := vault.Open(pass, vaultPathOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		pubPath, err := capturePubPath()
		if err != nil {
			return err
		}
		if _, err := capture.InitKeypair(v, pubPath, rotate); err != nil {
			return err
		}
		nonceP, err := captureNoncePath()
		if err != nil {
			return err
		}
		nonce, err := capture.EnsureNonce(nonceP)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "capture key ready (public: %s)\npairing nonce: %s\n", pubPath, nonce)
		return nil
	},
}

var captureHostCmd = &cobra.Command{
	Use:    "capture-host",
	Short:  "Native-messaging host for the Scout Capture extension (launched by the browser)",
	Hidden: true, // not a day-to-day command; the browser launches it
	RunE: func(cmd *cobra.Command, _ []string) error {
		extID, _ := cmd.Flags().GetString("ext-id")
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
		nonceP, err := captureNoncePath()
		if err != nil {
			return err
		}
		return capture.RunHost(cmd.InOrStdin(), cmd.OutOrStdout(), capture.HostConfig{
			Pub:          pub,
			SpoolDir:     spoolDir,
			AllowedExtID: extID,
			NoncePath:    nonceP,
		})
	},
}

var vaultImportCapturesCmd = &cobra.Command{
	Use:   "import-captures",
	Short: "Drain the capture spool into per-site vault profiles (review + confirm)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		pass, err := readPassphraseBytes(cmd.ErrOrStderr(), "Vault passphrase: ")
		if err != nil {
			return err
		}
		defer zeroBytesCLI(pass)
		v, err := vault.Open(pass, vaultPathOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		pubPath, err := capturePubPath()
		if err != nil {
			return err
		}
		pub, err := capture.LoadPub(pubPath)
		if err != nil {
			return err
		}
		priv, err := capture.LoadPriv(v)
		if err != nil {
			return err
		}
		spoolDir, err := capture.SpoolDir()
		if err != nil {
			return err
		}
		rep, err := capture.ImportSpool(v, spoolDir, pub, priv)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "imported %d capture(s) across %d site(s); %d quarantined\n",
			rep.Imported, len(rep.Sites), rep.Quarantined)
		return nil
	},
}

var captureHostInstallCmd = &cobra.Command{
	Use:   "install <extension-id>",
	Short: "Register the native-messaging host manifest for the given extension ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := installNativeManifest(args[0])
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed native-messaging manifest: %s\n", path)
		return nil
	},
}

var captureHostUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the native-messaging host manifest",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := uninstallNativeManifest(); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "removed native-messaging manifest")
		return nil
	},
}

func init() {
	vaultCaptureKeyInitCmd.Flags().Bool("rotate", false, "replace any existing capture keypair")
	vaultCaptureKeyInitCmd.Flags().String("vault-file", "", "override vault file path")
	vaultImportCapturesCmd.Flags().String("vault-file", "", "override vault file path")
	captureHostCmd.Flags().String("ext-id", "", "the extension ID permitted to connect")

	vaultCaptureKeyCmd.AddCommand(vaultCaptureKeyInitCmd)
	vaultCmd.AddCommand(vaultCaptureKeyCmd, vaultImportCapturesCmd)
	rootCmd.AddCommand(captureHostCmd)
	captureHostCmd.AddCommand(captureHostInstallCmd, captureHostUninstallCmd, captureHostKeygenCmd)
}
