package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout/archive"
)

// DownloadBrave downloads the latest Brave browser release from GitHub
// and extracts it to ~/.scout/browsers/brave/<version>/. Returns the
// path to the executable.
func DownloadBrave(ctx context.Context) (string, error) {
	version, err := latestBraveVersion(ctx)
	if err != nil {
		return "", err
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "brave", version)
	binPath := filepath.Join(destDir, braveBinPath())

	// Already downloaded.
	if FileExists(binPath) {
		RegisterBrowser("brave", version, binPath)
		return binPath, nil
	}

	dlURL := mustLoadManifest().Brave.DownloadURL(version)
	if dlURL == "" {
		return "", fmt.Errorf("scout: no Brave release available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	asset := braveAssetName(version)

	data, err := DownloadFile(ctx, dlURL)
	if err != nil {
		return "", fmt.Errorf("scout: download brave: %w", err)
	}

	// Clean and recreate dest dir.
	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("scout: clean brave dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create brave dir: %w", err)
	}

	if err := archive.Extract(data, asset, destDir); err != nil {
		return "", fmt.Errorf("scout: extract brave: %w", err)
	}

	// Make binary executable on Unix.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binPath, 0o755); err != nil {
			return "", fmt.Errorf("scout: chmod brave binary: %w", err)
		}
	}

	if !FileExists(binPath) {
		return "", fmt.Errorf("scout: brave binary not found at %s after extraction", binPath)
	}

	RegisterBrowser("brave", version, binPath)

	return binPath, nil
}

// latestBraveVersion fetches the latest Brave release tag from GitHub API.
func latestBraveVersion(ctx context.Context) (string, error) {
	apiURL := mustLoadManifest().Brave.BrowserAPI("latest_release")
	if apiURL == "" {
		return "", fmt.Errorf("scout: no Brave API URL in browser.json")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("scout: create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("scout: fetch brave version: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scout: github API returned HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("scout: decode github response: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("scout: empty tag_name in github response")
	}

	// Tag is "vX.Y.Z", strip the "v" prefix.
	return strings.TrimPrefix(release.TagName, "v"), nil
}

// braveAssetName returns the zip filename for the current platform and version from browser.json.
func braveAssetName(version string) string {
	return mustLoadManifest().Brave.ZipName(version)
}

// braveBinPath returns the relative path to the Brave executable from browser.json.
func braveBinPath() string {
	return mustLoadManifest().Brave.BinaryPath("brave")
}

// edgeBinPath returns the relative path to the Edge executable from browser.json.
func edgeBinPath() string {
	return mustLoadManifest().Edge.BinaryPath("msedge")
}

// DownloadEdge downloads Microsoft Edge Stable from the official updates API
// and extracts it to ~/.scout/browsers/edge/<version>/. Returns the path to the executable.
func DownloadEdge(ctx context.Context) (string, error) {
	if runtime.GOOS == "windows" {
		return downloadEdgeWindows(ctx)
	}

	return downloadEdgeUnix(ctx)
}

// downloadEdgeWindows copies the system-installed Edge into the browser cache.
// This uses lookupBrowser(Edge) intentionally — Windows has no standalone Edge
// download URL, so the only option is to copy from the system install path.
func downloadEdgeWindows(_ context.Context) (string, error) {
	systemPath, err := lookupBrowser(Edge)
	if err != nil {
		return "", fmt.Errorf("scout: edge not installed — download from https://www.microsoft.com/edge/download: %w", err)
	}

	appDir := filepath.Dir(systemPath)

	var version string

	entries, err := os.ReadDir(appDir)
	if err != nil {
		return "", fmt.Errorf("scout: read edge dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() && len(e.Name()) > 0 && e.Name()[0] >= '0' && e.Name()[0] <= '9' {
			version = e.Name()

			break
		}
	}

	if version == "" {
		return systemPath, nil
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "edge", version)
	binPath := filepath.Join(destDir, "msedge.exe")

	if FileExists(binPath) {
		RegisterBrowser("edge", version, binPath)
		return binPath, nil
	}

	srcDir := filepath.Join(appDir, version)

	if err := copyDir(srcDir, destDir); err != nil {
		return "", fmt.Errorf("scout: copy edge to cache: %w", err)
	}

	for _, name := range []string{"msedge.exe", "msedge.dll", "msedge_elf.dll"} {
		src := filepath.Join(appDir, name)
		if FileExists(src) {
			data, err := os.ReadFile(src)
			if err == nil {
				_ = os.WriteFile(filepath.Join(destDir, name), data, 0o755)
			}
		}
	}

	if !FileExists(binPath) {
		return systemPath, nil
	}

	_, _ = fmt.Fprintf(os.Stderr, "scout: cached Edge %s to %s\n", version, destDir)

	RegisterBrowser("edge", version, binPath)

	return binPath, nil
}

// downloadEdgeUnix downloads and extracts Edge for Linux/macOS.
func downloadEdgeUnix(ctx context.Context) (string, error) {
	version, dlURL, err := latestEdgeRelease(ctx)
	if err != nil {
		return "", err
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "edge", version)
	binPath := filepath.Join(destDir, edgeBinPath())

	if FileExists(binPath) {
		RegisterBrowser("edge", version, binPath)
		return binPath, nil
	}

	data, err := DownloadFile(ctx, dlURL)
	if err != nil {
		return "", fmt.Errorf("scout: download edge: %w", err)
	}

	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("scout: clean edge dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create edge dir: %w", err)
	}

	if err := extractEdge(data, dlURL, destDir); err != nil {
		return "", fmt.Errorf("scout: extract edge: %w", err)
	}

	if err := os.Chmod(binPath, 0o755); err != nil {
		return "", fmt.Errorf("scout: chmod edge binary: %w", err)
	}

	if !FileExists(binPath) {
		return "", fmt.Errorf("scout: edge binary not found at %s after extraction", binPath)
	}

	RegisterBrowser("edge", version, binPath)

	return binPath, nil
}

// latestEdgeRelease queries the Edge updates API for the latest Stable version and download URL.
func latestEdgeRelease(ctx context.Context) (version, downloadURL string, err error) {
	apiURL := mustLoadManifest().Edge.BrowserAPI("updates")
	if apiURL == "" {
		return "", "", fmt.Errorf("scout: no Edge updates API URL in browser.json")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("scout: create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("scout: fetch edge updates: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("scout: edge updates API returned HTTP %d", resp.StatusCode)
	}

	var products []struct {
		Product  string `json:"Product"`
		Releases []struct {
			Platform       string `json:"Platform"`
			Architecture   string `json:"Architecture"`
			ProductVersion string `json:"ProductVersion"`
			Artifacts      []struct {
				ArtifactName string `json:"ArtifactName"`
				Location     string `json:"Location"`
			} `json:"Artifacts"`
		} `json:"Releases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return "", "", fmt.Errorf("scout: decode edge updates: %w", err)
	}

	wantPlatform, wantArch := edgePlatformArch()

	for _, p := range products {
		if p.Product != "Stable" {
			continue
		}

		for _, r := range p.Releases {
			if !strings.EqualFold(r.Platform, wantPlatform) || !strings.EqualFold(r.Architecture, wantArch) {
				continue
			}

			for _, a := range r.Artifacts {
				if isEdgeArtifact(a.ArtifactName) {
					return r.ProductVersion, a.Location, nil
				}
			}
		}
	}

	return "", "", fmt.Errorf("scout: no Edge Stable release for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// edgePlatformArch maps Go runtime to Edge API platform/architecture strings.
func edgePlatformArch() (platform, arch string) {
	switch runtime.GOOS {
	case "windows":
		platform = "Windows"
	case "darwin":
		platform = "MacOS"
	case "linux":
		platform = "Linux"
	default:
		platform = runtime.GOOS
	}

	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	case "386":
		arch = "x86"
	default:
		arch = runtime.GOARCH
	}

	if runtime.GOOS == "darwin" {
		arch = "universal"
	}

	return platform, arch
}

// isEdgeArtifact returns true for the artifact type we can extract on this OS.
func isEdgeArtifact(name string) bool {
	switch runtime.GOOS {
	case "windows":
		return name == "msi"
	case "linux":
		return name == "deb"
	case "darwin":
		return name == "pkg"
	default:
		return false
	}
}

// extractEdge extracts the Edge installer based on file type.
func extractEdge(data []byte, dlURL, destDir string) error {
	lower := strings.ToLower(dlURL)

	switch {
	case strings.HasSuffix(lower, ".msi"):
		return extractMSI(data, destDir)
	case strings.HasSuffix(lower, ".deb"):
		return archive.Extract(data, "edge.deb", destDir)
	case strings.HasSuffix(lower, ".pkg"):
		return extractMacPKG(data, destDir)
	default:
		return fmt.Errorf("unsupported edge installer format: %s", filepath.Base(dlURL))
	}
}

// extractMSI extracts an MSI archive.
func extractMSI(data []byte, destDir string) error {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("scout-edge-%d.msi", time.Now().UnixNano()))

	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("write temp msi: %w", err)
	}

	defer func() { _ = os.Remove(tmpFile) }()

	if sevenZip, err := exec.LookPath("7z"); err == nil {
		cmd := exec.Command(sevenZip, "x", "-y", "-o"+destDir, tmpFile)

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("7z extract msi: %w\n%s", err, string(output))
		}

		return nil
	}

	if runtime.GOOS != "windows" {
		return fmt.Errorf("7z not found — install 7-Zip to extract MSI on this platform")
	}

	cmdLine := fmt.Sprintf(`msiexec /a "%s" /qn TARGETDIR="%s"`, filepath.Clean(tmpFile), filepath.Clean(destDir))
	cmd := exec.Command("cmd", "/c", cmdLine)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("msiexec extract: %w\n%s", err, string(output))
	}

	_ = os.Remove(filepath.Join(destDir, filepath.Base(tmpFile)))

	return nil
}

// extractMacPKG extracts a macOS .pkg using pkgutil on macOS.
func extractMacPKG(data []byte, destDir string) error {
	tmpFile := filepath.Join(os.TempDir(), "scout-edge-"+fmt.Sprintf("%d", time.Now().UnixNano())+".pkg")

	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("write temp pkg: %w", err)
	}

	defer func() { _ = os.Remove(tmpFile) }()

	cmd := exec.Command("pkgutil", "--expand-full", tmpFile, destDir)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pkgutil extract: %w\n%s", err, string(output))
	}

	return nil
}
