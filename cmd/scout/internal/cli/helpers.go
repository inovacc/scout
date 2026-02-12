package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"

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
func readPassphrase(w io.Writer, prompt string) (string, error) {
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

	if pass1 != pass2 {
		return "", fmt.Errorf("scout: passphrases do not match")
	}

	return pass1, nil
}

// truncate truncates a string to maxLen, appending "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen-3] + "..."
}
