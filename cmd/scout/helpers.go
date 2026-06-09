package main

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"io"
	"os"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// writeOutput writes raw bytes to the output file or stdout.
func writeOutput(cmd *cobra.Command, data []byte, defaultName string) (string, error) {
	outFile, _ := cmd.Flags().GetString("output")
	if outFile == "" {
		outFile = defaultName
	}

	if outFile == "-" {
		_, err := cmd.OutOrStdout().Write(data)
		return "stdout", err
	}

	if err := os.WriteFile(outFile, data, 0o644); err != nil {
		return "", fmt.Errorf("scout: write file: %w", err)
	}

	return outFile, nil
}

// readPassphrase prompts for a passphrase with echo disabled.
// If SCOUT_PASSPHRASE is set, it is used without prompting.
func readPassphrase(w io.Writer, prompt string) (string, error) {
	if v := os.Getenv("SCOUT_PASSPHRASE"); v != "" {
		return v, nil
	}

	_, _ = fmt.Fprint(w, prompt)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		_, _ = fmt.Fprintln(w)

		if err != nil {
			return "", fmt.Errorf("scout: read password: %w", err)
		}

		return string(b), nil
	}

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text(), nil
	}

	return "", fmt.Errorf("scout: no passphrase provided")
}

// readPassphraseConfirm reads a passphrase twice and verifies they match.
func readPassphraseConfirm(w io.Writer) (string, error) {
	pass1, err := readPassphrase(w, "Enter passphrase: ")
	if err != nil {
		return "", err
	}

	if pass1 == "" {
		return "", fmt.Errorf("scout: passphrase cannot be empty")
	}

	pass2, err := readPassphrase(w, "Confirm passphrase: ")
	if err != nil {
		return "", err
	}

	if subtle.ConstantTimeCompare([]byte(pass1), []byte(pass2)) != 1 {
		return "", fmt.Errorf("scout: passphrases do not match")
	}

	return pass1, nil
}

// isHeadless reads the --headless persistent flag from the command.
func isHeadless(cmd *cobra.Command) bool {
	h, _ := cmd.Flags().GetBool("headless")

	return h
}

// browserOpt returns a WithBrowser option from the --browser persistent flag.
func browserOpt(cmd *cobra.Command) scout.Option {
	b, _ := cmd.Flags().GetString("browser")
	return scout.WithBrowser(scout.BrowserType(b))
}

// stealthOpts returns WithStealth if the --stealth flag is set, or nil.
func stealthOpts(cmd *cobra.Command) []scout.Option {
	s, _ := cmd.Flags().GetBool("stealth")
	if s {
		return []scout.Option{scout.WithStealth()}
	}

	return nil
}

// baseOpts returns the common browser options derived from persistent flags.
func baseOpts(cmd *cobra.Command) []scout.Option {
	opts := []scout.Option{
		scout.WithHeadless(isHeadless(cmd)),
		scout.WithNoSandbox(),
		browserOpt(cmd),
	}
	opts = append(opts, stealthOpts(cmd)...)

	if v, _ := cmd.Flags().GetString("user-data-dir"); v != "" {
		opts = append(opts, scout.WithUserDataDir(v))
	}

	if v, _ := cmd.Flags().GetString("profile-directory"); v != "" {
		opts = append(opts, scout.WithLaunchFlag("profile-directory", v))
	}

	if v, _ := cmd.Flags().GetBool("system-browser"); v {
		opts = append(opts, scout.WithSystemBrowser())
	}

	if v, _ := cmd.Flags().GetString("electron-app"); v != "" {
		opts = append(opts, scout.WithElectronApp(v))
	}

	if v, _ := cmd.Flags().GetString("electron-version"); v != "" {
		opts = append(opts, scout.WithElectronVersion(v))
	}

	if v, _ := cmd.Flags().GetString("electron-cdp"); v != "" {
		opts = append(opts, scout.WithElectronCDP(v))
	}

	return opts
}

// readPassphraseBytes reads a passphrase as a zeroable []byte. It honors
// SCOUT_VAULT_PASSPHRASE (with a stderr leak warning) for non-interactive use,
// then falls back to a no-echo terminal prompt, then a piped line.
func readPassphraseBytes(w io.Writer, prompt string) ([]byte, error) {
	if v := os.Getenv("SCOUT_VAULT_PASSPHRASE"); v != "" {
		_, _ = fmt.Fprintln(w, "warning: SCOUT_VAULT_PASSPHRASE is visible to child processes; prefer the interactive prompt")
		return []byte(v), nil
	}
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		_, _ = fmt.Fprint(w, prompt)
		b, err := term.ReadPassword(int(f.Fd()))
		_, _ = fmt.Fprintln(w)
		if err != nil {
			return nil, fmt.Errorf("scout: read passphrase: %w", err)
		}
		return b, nil
	}
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return nil, fmt.Errorf("scout: read passphrase: no input")
	}
	return append([]byte(nil), sc.Bytes()...), nil
}

// truncate truncates a string to maxLen, appending "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen-3] + "..."
}
