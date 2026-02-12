package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/inovacc/scout"
	"github.com/inovacc/scout/scraper"
	"github.com/inovacc/scout/scraper/slack"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "capture":
		cmdCapture(os.Args[2:])
	case "load":
		cmdLoad(os.Args[2:])
	case "decrypt":
		cmdDecrypt(os.Args[2:])
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])

		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	_, _ = fmt.Fprintf(os.Stderr, `Usage: slack-assist <command> [flags]

Commands:
  capture   Launch browser, log in to Slack, capture and encrypt session
  load      Load and display info from an encrypted session file
  decrypt   Decrypt session file to plaintext JSON

Run 'slack-assist <command> -help' for details.
`)
}

func cmdCapture(args []string) {
	fs := flag.NewFlagSet("capture", flag.ExitOnError)
	workspace := fs.String("workspace", "", "Slack workspace domain (e.g. myteam.slack.com or myteam)")
	output := fs.String("output", "slack-session.enc", "output file path for encrypted session")
	timeout := fs.Duration("timeout", 5*time.Minute, "max time to wait for login")

	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}

	if *workspace == "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: -workspace is required")

		fs.Usage()
		os.Exit(1)
	}

	// Read and confirm passphrase
	passphrase, err := readPassphraseConfirm()
	if err != nil {
		log.Fatalf("passphrase: %v", err)
	}

	session, err := captureSession(*workspace, *timeout, passphrase, *output)
	if err != nil {
		log.Fatalf("%v", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "\nsession saved to: %s\n", *output)
	_, _ = fmt.Fprintf(os.Stdout, "  workspace: %s\n", session.WorkspaceURL)
	_, _ = fmt.Fprintf(os.Stdout, "  token:     %s...\n", session.Token[:min(20, len(session.Token))])
	_, _ = fmt.Fprintf(os.Stdout, "  cookies:   %d\n", len(session.Cookies))
	_, _ = fmt.Fprintf(os.Stdout, "  timestamp: %s\n", session.Timestamp.Format(time.RFC3339))
}

func captureSession(workspace string, timeout time.Duration, passphrase, output string) (*slack.CapturedSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		_ = sig

		_, _ = fmt.Fprintln(os.Stderr, "\ninterrupted, shutting down...")

		cancel()
	}()

	session, err := runCapture(ctx, workspace, timeout)
	if err != nil {
		return nil, err
	}

	if err := slack.SaveEncrypted(session, output, passphrase); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return session, nil
}

func runCapture(ctx context.Context, workspace string, timeout time.Duration) (*slack.CapturedSession, error) {
	_, _ = fmt.Fprintln(os.Stdout, "launching browser...")

	browser, err := scout.New(
		scout.WithHeadless(false),
		scout.WithNoSandbox(),
		scout.WithTimeout(timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("launch browser: %w", err)
	}

	defer func() { _ = browser.Close() }()

	workspaceURL := normalizeWorkspaceURL(workspace)

	_, _ = fmt.Fprintf(os.Stdout, "navigating to %s\n", workspaceURL)

	page, err := browser.NewPage(workspaceURL)
	if err != nil {
		return nil, fmt.Errorf("open page: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, "please log in to Slack in the browser window...")
	_, _ = fmt.Fprintln(os.Stdout, "waiting for xoxc token (polling every 2s)...")

	// Poll for token
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout: could not detect login within %v", timeout)
		case <-ticker.C:
			result, err := page.Eval(tokenExtractionJS)
			if err != nil {
				continue
			}

			token := result.String()
			if token == "" || !strings.HasPrefix(token, "xoxc-") {
				continue
			}

			_, _ = fmt.Fprintln(os.Stdout, "token detected! capturing session...")

			session, err := slack.CaptureFromPage(page)
			if err != nil {
				return nil, fmt.Errorf("capture session: %w", err)
			}

			return session, nil
		}
	}
}

func cmdLoad(args []string) {
	fs := flag.NewFlagSet("load", flag.ExitOnError)
	input := fs.String("input", "slack-session.enc", "encrypted session file path")

	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}

	passphrase, err := readPassphrase("Enter passphrase: ")
	if err != nil {
		log.Fatalf("passphrase: %v", err)
	}

	session, err := slack.LoadEncrypted(*input, passphrase)
	if err != nil {
		log.Fatalf("load session: %v", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, "session info:")
	_, _ = fmt.Fprintf(os.Stdout, "  version:    %s\n", session.Version)
	_, _ = fmt.Fprintf(os.Stdout, "  workspace:  %s\n", session.WorkspaceURL)
	_, _ = fmt.Fprintf(os.Stdout, "  token:      %s...\n", session.Token[:min(20, len(session.Token))])
	_, _ = fmt.Fprintf(os.Stdout, "  d_cookie:   %s...\n", session.DCookie[:min(20, len(session.DCookie))])
	_, _ = fmt.Fprintf(os.Stdout, "  cookies:    %d\n", len(session.Cookies))
	_, _ = fmt.Fprintf(os.Stdout, "  localStorage keys: %d\n", len(session.LocalStorage))
	_, _ = fmt.Fprintf(os.Stdout, "  sessionStorage keys: %d\n", len(session.SessionStorage))
	_, _ = fmt.Fprintf(os.Stdout, "  timestamp:  %s\n", session.Timestamp.Format(time.RFC3339))

	if session.UserID != "" {
		_, _ = fmt.Fprintf(os.Stdout, "  user_id:    %s\n", session.UserID)
	}

	if session.TeamName != "" {
		_, _ = fmt.Fprintf(os.Stdout, "  team:       %s\n", session.TeamName)
	}
}

func cmdDecrypt(args []string) {
	fs := flag.NewFlagSet("decrypt", flag.ExitOnError)
	input := fs.String("input", "slack-session.enc", "encrypted session file path")
	output := fs.String("output", "slack-session.json", "plaintext JSON output path")

	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}

	passphrase, err := readPassphrase("Enter passphrase: ")
	if err != nil {
		log.Fatalf("passphrase: %v", err)
	}

	session, err := slack.LoadEncrypted(*input, passphrase)
	if err != nil {
		log.Fatalf("load session: %v", err)
	}

	if err := scraper.ExportJSON(session, *output); err != nil {
		log.Fatalf("export json: %v", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "decrypted session written to: %s\n", *output)
	_, _ = fmt.Fprintln(os.Stderr, "WARNING: this file contains sensitive credentials in plaintext")
}

// readPassphrase prompts for a passphrase with echo disabled.
func readPassphrase(prompt string) (string, error) {
	_, _ = fmt.Fprint(os.Stderr, prompt)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)

		_, _ = fmt.Fprintln(os.Stderr)

		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}

		return string(b), nil
	}

	// Fallback for piped input
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text(), nil
	}

	return "", fmt.Errorf("no passphrase provided")
}

// readPassphraseConfirm reads a passphrase twice and verifies they match.
func readPassphraseConfirm() (string, error) {
	pass1, err := readPassphrase("Enter passphrase: ")
	if err != nil {
		return "", err
	}

	if pass1 == "" {
		return "", fmt.Errorf("passphrase cannot be empty")
	}

	pass2, err := readPassphrase("Confirm passphrase: ")
	if err != nil {
		return "", err
	}

	if pass1 != pass2 {
		return "", fmt.Errorf("passphrases do not match")
	}

	return pass1, nil
}

// normalizeWorkspaceURL ensures the workspace URL is fully qualified.
// Duplicated from auth.go to keep the CLI self-contained within its import graph.
func normalizeWorkspaceURL(workspace string) string {
	if strings.HasPrefix(workspace, "http://") || strings.HasPrefix(workspace, "https://") {
		return workspace
	}

	if !strings.Contains(workspace, ".") {
		workspace += ".slack.com"
	}

	return "https://" + workspace
}

// tokenExtractionJS is the same JS used by the slack scraper to find xoxc tokens.
// Duplicated here to avoid exporting internal constants from the slack package.
const tokenExtractionJS = `() => {
	// Try localConfig_v2
	try {
		const keys = Object.keys(localStorage);
		for (const key of keys) {
			if (key.startsWith('localConfig_v2')) {
				const val = localStorage.getItem(key);
				if (val) {
					const parsed = JSON.parse(val);
					const teams = parsed.teams || {};
					for (const teamId of Object.keys(teams)) {
						const token = teams[teamId].token;
						if (token && token.startsWith('xoxc-')) {
							return token;
						}
					}
				}
			}
		}
	} catch(e) {}

	// Try window.boot_data
	try {
		if (window.boot_data && window.boot_data.api_token) {
			const token = window.boot_data.api_token;
			if (token.startsWith('xoxc-')) {
				return token;
			}
		}
	} catch(e) {}

	// Try all localStorage keys for xoxc pattern
	try {
		for (let i = 0; i < localStorage.length; i++) {
			const key = localStorage.key(i);
			const val = localStorage.getItem(key);
			if (val && val.includes('xoxc-')) {
				const match = val.match(/(xoxc-[a-zA-Z0-9-]+)/);
				if (match) return match[1];
			}
		}
	} catch(e) {}

	return "";
}`
