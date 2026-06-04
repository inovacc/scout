package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.AddCommand(updateCheckCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update scout to the latest release",
	Long: `Download and install the latest scout release from GitHub.

Checks https://api.github.com/repos/inovacc/scout/releases/latest for the
newest version, compares it with the current build, and replaces the running
binary if a newer version is available.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		current := Version
		out := cmd.OutOrStdout()

		_, _ = fmt.Fprintf(out, "Current version: %s\n", current)
		_, _ = fmt.Fprintf(out, "Checking for updates...\n")

		release, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("scout: update: %w", err)
		}

		_, _ = fmt.Fprintf(out, "Latest version:  %s\n", release.TagName)

		if !isNewer(current, release.TagName) {
			_, _ = fmt.Fprintf(out, "Already up to date.\n")
			return nil
		}

		assetName := buildAssetName()
		assetURL := ""

		for _, a := range release.Assets {
			if a.Name == assetName {
				assetURL = a.BrowserDownloadURL
				break
			}
		}

		if assetURL == "" {
			return fmt.Errorf("scout: update: no release asset %q found for %s", assetName, release.TagName)
		}

		// Resolve the expected checksum from the release's checksums.txt and
		// refuse to install a binary we cannot verify (supply-chain integrity).
		checksumsURL := ""

		for _, a := range release.Assets {
			if a.Name == "checksums.txt" {
				checksumsURL = a.BrowserDownloadURL
				break
			}
		}

		if checksumsURL == "" {
			return fmt.Errorf("scout: update: release %s has no checksums.txt; refusing to install an unverified binary", release.TagName)
		}

		expectedSHA, err := fetchChecksum(checksumsURL, assetName)
		if err != nil {
			return fmt.Errorf("scout: update: %w", err)
		}

		_, _ = fmt.Fprintf(out, "Downloading %s ...\n", assetURL)

		if err := selfReplace(assetURL, expectedSHA); err != nil {
			return fmt.Errorf("scout: update: %w", err)
		}

		_, _ = fmt.Fprintf(out, "Updated successfully to %s\n", release.TagName)

		return nil
	},
}

var updateCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if a newer version is available",
	RunE: func(cmd *cobra.Command, _ []string) error {
		current := Version
		out := cmd.OutOrStdout()

		_, _ = fmt.Fprintf(out, "Current version: %s\n", current)

		release, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("scout: update: check: %w", err)
		}

		_, _ = fmt.Fprintf(out, "Latest version:  %s\n", release.TagName)

		if isNewer(current, release.TagName) {
			_, _ = fmt.Fprintf(out, "Update available! Run 'scout update' to install.\n")
		} else {
			_, _ = fmt.Fprintf(out, "Already up to date.\n")
		}

		return nil
	},
}

// githubRelease is the subset of the GitHub release JSON we care about.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

const releaseURL = "https://api.github.com/repos/inovacc/scout/releases/latest"

func fetchLatestRelease() (*githubRelease, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("scout: update: build request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scout: update: fetch release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scout: update: github api returned %s", resp.Status)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("scout: update: decode release json: %w", err)
	}

	return &rel, nil
}

// buildAssetName returns the expected asset filename for the current platform.
// Convention: scout-{os}-{arch}[.exe]
func buildAssetName() string {
	name := fmt.Sprintf("scout-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return name
}

// isNewer reports whether the remote release tag is strictly newer than the
// local version. Dev/empty builds are always updatable. Comparison is
// semver-aware so `scout update` refuses to "update" to an older or equal
// release — a downgrade attack (a rolled-back or spoofed older tag served by a
// hostile/compromised release endpoint) would otherwise re-introduce patched
// vulnerabilities. Set SCOUT_ALLOW_DOWNGRADE=1 (or =true) to permit a
// deliberate rollback to any different tag.
func isNewer(current, remote string) bool {
	cv := strings.TrimPrefix(current, "v")
	rv := strings.TrimPrefix(remote, "v")

	// Dev builds are always "updatable".
	if cv == "dev" || cv == "" {
		return true
	}

	// Explicit operator opt-in: allow rollback to any different tag.
	if v := os.Getenv("SCOUT_ALLOW_DOWNGRADE"); v == "1" || strings.EqualFold(v, "true") {
		return cv != rv
	}

	c, r := "v"+cv, "v"+rv
	if !semver.IsValid(c) || !semver.IsValid(r) {
		// Conservative fallback for non-semver tags: update only on a
		// difference (preserves prior behavior for odd version strings).
		return cv != rv
	}

	// Strictly newer only — refuse equal or older.
	return semver.Compare(r, c) > 0
}

// maxBinaryDownload caps the self-update download to defend against an oversized
// (memory/disk-exhausting) response.
const maxBinaryDownload = 256 << 20 // 256 MB

// secureHTTPClient refuses any redirect to a non-https target, preventing a
// TLS-downgrade on the update download path.
func secureHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("scout: update: refusing non-https redirect to %q", req.URL.Scheme)
			}

			return nil
		},
	}
}

// fetchChecksum downloads the release checksums.txt and returns the expected hex
// SHA256 for assetName (goreleaser format: "<hex>  <name>" per line).
func fetchChecksum(checksumsURL, assetName string) (string, error) {
	if !strings.HasPrefix(checksumsURL, "https://") {
		return "", fmt.Errorf("scout: update: checksums url is not https")
	}

	resp, err := secureHTTPClient(30 * time.Second).Get(checksumsURL) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("scout: update: fetch checksums: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scout: update: checksums returned %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB is ample for checksums.txt
	if err != nil {
		return "", fmt.Errorf("scout: update: read checksums: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			return fields[0], nil
		}
	}

	return "", fmt.Errorf("scout: update: no checksum entry for %q in checksums.txt", assetName)
}

// selfReplace downloads the binary from url, verifies it against expectedSHA, and
// atomically replaces the running executable. It FAILS CLOSED: a non-https url, a
// missing/incorrect checksum, or an oversized body aborts the update before any
// bytes are written over the live binary.
func selfReplace(url, expectedSHA string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("scout: update: refusing non-https download url")
	}

	if strings.TrimSpace(expectedSHA) == "" {
		return fmt.Errorf("scout: update: missing expected checksum; refusing unverified update")
	}

	resp, err := secureHTTPClient(5 * time.Minute).Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("scout: update: download binary: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("scout: update: download returned %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBinaryDownload+1))
	if err != nil {
		return fmt.Errorf("scout: update: download binary: %w", err)
	}

	if len(data) > maxBinaryDownload {
		return fmt.Errorf("scout: update: download exceeds %d-byte limit", maxBinaryDownload)
	}

	// Integrity gate: verify BEFORE the bytes ever touch the executable path.
	if got := sha256Hex(data); !strings.EqualFold(got, strings.TrimSpace(expectedSHA)) {
		return fmt.Errorf("scout: update: checksum mismatch: expected %s, got %s", expectedSHA, got)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("scout: update: locate current executable: %w", err)
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("scout: update: resolve executable path: %w", err)
	}

	// Write to a temp file in the same directory (ensures same filesystem for rename).
	dir := filepath.Dir(exe)

	tmp, err := os.CreateTemp(dir, "scout-update-*.tmp")
	if err != nil {
		return fmt.Errorf("scout: update: create temp file: %w", err)
	}

	tmpPath := tmp.Name()

	defer func() { _ = os.Remove(tmpPath) }() // clean up on failure

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("scout: update: write temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("scout: update: close temp file: %w", err)
	}

	// Make executable on Unix.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			return fmt.Errorf("scout: update: chmod: %w", err)
		}
	}

	// Platform-specific replacement.
	if runtime.GOOS == "windows" {
		return selfReplaceWindows(exe, tmpPath)
	}

	return selfReplaceUnix(exe, tmpPath)
}

// selfReplaceUnix atomically renames the temp file over the current executable.
func selfReplaceUnix(exe, tmpPath string) error {
	if err := os.Rename(tmpPath, exe); err != nil {
		return fmt.Errorf("scout: update: rename over executable: %w", err)
	}

	return nil
}

// selfReplaceWindows works around the Windows file-lock limitation:
// 1. Rename current executable to .old
// 2. Rename new binary into place
// 3. Best-effort remove .old
func selfReplaceWindows(exe, tmpPath string) error {
	oldPath := exe + ".old"

	// Remove leftover .old from a previous update.
	_ = os.Remove(oldPath)

	if err := os.Rename(exe, oldPath); err != nil {
		return fmt.Errorf("scout: update: rename current to .old: %w", err)
	}

	if err := os.Rename(tmpPath, exe); err != nil {
		// Try to restore the original.
		_ = os.Rename(oldPath, exe)
		return fmt.Errorf("scout: update: rename new binary into place: %w", err)
	}

	// Best-effort cleanup; the file may still be locked.
	_ = os.Remove(oldPath)

	return nil
}
