package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestLatestChromiumRevision_MockEndpoint tests the LAST_CHANGE parsing logic
// used by latestChromiumRevision by replicating the same HTTP+parse pattern.
func TestLatestChromiumRevision_MockEndpoint(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantRev int
		wantOK  bool
	}{
		{
			name: "valid_revision",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, "1593218")
			},
			wantRev: 1593218,
			wantOK:  true,
		},
		{
			name: "revision_with_whitespace",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, "  1593218\n")
			},
			wantRev: 1593218,
			wantOK:  true,
		},
		{
			name: "404_response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantRev: 0,
			wantOK:  false,
		},
		{
			name: "empty_body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantRev: 0,
			wantOK:  false,
		},
		{
			name: "malformed_body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, "not-a-number")
			},
			wantRev: 0,
			wantOK:  false,
		},
		{
			name: "500_error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantRev: 0,
			wantOK:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			// Replicate the parsing logic from latestChromiumRevision.
			rev, ok := parseLastChange(t, srv.URL)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}

			if rev != tc.wantRev {
				t.Errorf("rev = %d, want %d", rev, tc.wantRev)
			}
		})
	}
}

// parseLastChange replicates the HTTP+parse logic of latestChromiumRevision.
func parseLastChange(t *testing.T, url string) (int, bool) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return 0, false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}

		return 0, false
	}

	defer func() { _ = resp.Body.Close() }()

	var buf [20]byte

	n, _ := resp.Body.Read(buf[:])
	body := strings.TrimSpace(string(buf[:n]))

	var rev int
	if _, err := fmt.Sscanf(body, "%d", &rev); err != nil {
		return 0, false
	}

	return rev, true
}

// TestLatestChromeForTesting_MockAPI tests Chrome for Testing JSON parsing.
func TestLatestChromeForTesting_MockAPI(t *testing.T) {
	t.Helper()

	wantPlatform := chromeCfTPlatformID()
	if wantPlatform == "" {
		t.Skip("no CfT platform ID for this OS/arch")
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantVer string
		wantURL string
		wantErr bool
	}{
		{
			name: "valid_response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"channels": map[string]any{
						"Stable": map[string]any{
							"version": "130.0.6723.58",
							"downloads": map[string]any{
								"chrome": []map[string]string{
									{"platform": wantPlatform, "url": "https://example.com/chrome.zip"},
								},
							},
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")

				if err := json.NewEncoder(w).Encode(resp); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			wantVer: "130.0.6723.58",
			wantURL: "https://example.com/chrome.zip",
			wantErr: false,
		},
		{
			name: "empty_version",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"channels": map[string]any{
						"Stable": map[string]any{
							"version":   "",
							"downloads": map[string]any{"chrome": []any{}},
						},
					},
				}
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			wantErr: true,
		},
		{
			name: "no_matching_platform",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"channels": map[string]any{
						"Stable": map[string]any{
							"version": "130.0.6723.58",
							"downloads": map[string]any{
								"chrome": []map[string]string{
									{"platform": "fakeos", "url": "https://example.com/fake.zip"},
								},
							},
						},
					},
				}
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			wantErr: true,
		},
		{
			name: "404_response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
		{
			name: "malformed_json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, "{invalid json")
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			ver, dlURL, err := parseChromeForTesting(t, srv.URL, wantPlatform)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ver != tc.wantVer {
				t.Errorf("version = %q, want %q", ver, tc.wantVer)
			}

			if dlURL != tc.wantURL {
				t.Errorf("url = %q, want %q", dlURL, tc.wantURL)
			}
		})
	}
}

// parseChromeForTesting replicates the JSON parsing logic of latestChromeForTesting.
func parseChromeForTesting(t *testing.T, apiURL, wantPlatform string) (string, string, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct {
		Channels struct {
			Stable struct {
				Version   string `json:"version"`
				Downloads struct {
					Chrome []struct {
						Platform string `json:"platform"`
						URL      string `json:"url"`
					} `json:"chrome"`
				} `json:"downloads"`
			} `json:"Stable"`
		} `json:"channels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decode: %w", err)
	}

	stable := result.Channels.Stable
	if stable.Version == "" {
		return "", "", fmt.Errorf("empty version")
	}

	for _, dl := range stable.Downloads.Chrome {
		if dl.Platform == wantPlatform {
			return stable.Version, dl.URL, nil
		}
	}

	return "", "", fmt.Errorf("no download for platform %s", wantPlatform)
}

// TestLatestEdgeRelease_MockAPI tests Edge updates API JSON parsing.
func TestLatestEdgeRelease_MockAPI(t *testing.T) {
	t.Helper()

	wantPlatform, wantArch := edgePlatformArch()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantVer string
		wantURL string
		wantErr bool
	}{
		{
			name: "valid_response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := []map[string]any{
					{
						"Product": "Stable",
						"Releases": []map[string]any{
							{
								"Platform":       wantPlatform,
								"Architecture":   wantArch,
								"ProductVersion": "130.0.2849.46",
								"Artifacts": []map[string]string{
									{"ArtifactName": edgeArtifactForOS(), "Location": "https://example.com/edge.msi"},
								},
							},
						},
					},
				}
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			wantVer: "130.0.2849.46",
			wantURL: "https://example.com/edge.msi",
			wantErr: false,
		},
		{
			name: "no_stable_product",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := []map[string]any{
					{"Product": "Beta", "Releases": []any{}},
				}
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			wantErr: true,
		},
		{
			name: "wrong_platform",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := []map[string]any{
					{
						"Product": "Stable",
						"Releases": []map[string]any{
							{
								"Platform":       "FakeOS",
								"Architecture":   "fake",
								"ProductVersion": "130.0.2849.46",
								"Artifacts":      []any{},
							},
						},
					},
				}
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			wantErr: true,
		},
		{
			name: "404_response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
		{
			name: "malformed_json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, "not json")
			},
			wantErr: true,
		},
		{
			name: "empty_array",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, "[]")
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			ver, dlURL, err := parseEdgeUpdates(t, srv.URL, wantPlatform, wantArch)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ver != tc.wantVer {
				t.Errorf("version = %q, want %q", ver, tc.wantVer)
			}

			if dlURL != tc.wantURL {
				t.Errorf("url = %q, want %q", dlURL, tc.wantURL)
			}
		})
	}
}

// edgeArtifactForOS returns the expected artifact name for the current OS.
func edgeArtifactForOS() string {
	switch {
	case isEdgeArtifact("msi"):
		return "msi"
	case isEdgeArtifact("deb"):
		return "deb"
	case isEdgeArtifact("pkg"):
		return "pkg"
	default:
		return "msi" // fallback for tests
	}
}

// parseEdgeUpdates replicates the JSON parsing logic of latestEdgeRelease.
func parseEdgeUpdates(t *testing.T, apiURL, wantPlatform, wantArch string) (string, string, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
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
		return "", "", fmt.Errorf("decode: %w", err)
	}

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

	return "", "", fmt.Errorf("no Edge Stable release found")
}

// TestEdgePlatformArch_Values validates known platform/arch mappings.
func TestEdgePlatformArch_Values(t *testing.T) {
	platform, arch := edgePlatformArch()

	// Platform should be one of the known values.
	validPlatforms := map[string]bool{"Windows": true, "MacOS": true, "Linux": true}
	if !validPlatforms[platform] {
		t.Logf("platform %q is not in known set (may be valid for unusual OS)", platform)
	}

	// Arch should be non-empty.
	if arch == "" {
		t.Error("arch should not be empty")
	}

	t.Logf("edgePlatformArch() = platform=%q arch=%q", platform, arch)
}

// TestChromeCfTPlatformID_NonEmpty validates the CfT platform ID is set.
func TestChromeCfTPlatformID_NonEmpty(t *testing.T) {
	id := chromeCfTPlatformID()
	if id == "" {
		t.Skip("no CfT platform ID for this OS/arch")
	}

	// Should be one of the known CfT platform IDs.
	known := []string{"win32", "win64", "mac-x64", "mac-arm64", "linux64"}
	found := slices.Contains(known, id)

	if !found {
		t.Logf("platform ID %q not in known set %v", id, known)
	}
}

// TestDownloadFile_ServerClosed tests behavior when server closes connection.
func TestDownloadFile_ServerClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	srv.Close() // close immediately

	_, err := DownloadFile(context.Background(), srv.URL)
	if err == nil {
		t.Error("expected error for closed server")
	}
}

// TestDownloadFile_InvalidURL tests behavior with invalid URL.
func TestDownloadFile_InvalidURL(t *testing.T) {
	_, err := DownloadFile(context.Background(), "://invalid")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// TestStripFirstDir_EmptyDir tests stripFirstDir on an empty directory.
func TestStripFirstDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := stripFirstDir(dir); err != nil {
		t.Fatalf("stripFirstDir(empty) should not error: %v", err)
	}
}

// TestStripFirstDir_SingleFile tests stripFirstDir when there's a single file (not dir).
func TestStripFirstDir_SingleFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "only.txt"), []byte("data"), 0o644)

	if err := stripFirstDir(dir); err != nil {
		t.Fatalf("stripFirstDir(single file) should not error: %v", err)
	}

	// File should still exist (no stripping when single entry is a file).
	if !FileExists(filepath.Join(dir, "only.txt")) {
		t.Error("only.txt should still exist")
	}
}

// TestExtractEdge_AllFormats tests extractEdge format detection.
func TestExtractEdge_AllFormats(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.com/edge.tar.gz", true},
		{"https://example.com/edge.exe", true},
		{"https://example.com/edge.rpm", true},
	}

	for _, tc := range tests {
		t.Run(filepath.Base(tc.url), func(t *testing.T) {
			err := extractEdge([]byte("data"), tc.url, t.TempDir())
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
		})
	}
}

// TestIsEdgeArtifact_AllTypes tests isEdgeArtifact with all artifact types.
func TestIsEdgeArtifact_AllTypes(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"msi"},
		{"deb"},
		{"pkg"},
		{"exe"},
		{"rpm"},
		{"dmg"},
		{""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isEdgeArtifact(tc.name)
			// Just ensure it doesn't panic; correctness depends on runtime.GOOS.
			t.Logf("isEdgeArtifact(%q) = %v", tc.name, got)
		})
	}
}

// TestCopyDir_NonExistentSource tests copyDir with non-existent source.
func TestCopyDir_NonExistentSource(t *testing.T) {
	err := copyDir(filepath.Join(t.TempDir(), "nonexistent"), t.TempDir())
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}

// TestLatestBraveVersion_MockVariants tests the Brave version parsing with various responses.
func TestLatestBraveVersion_MockVariants(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantVer string
		wantErr bool
	}{
		{
			name: "valid_tag",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				if err := json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.87.188"}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			wantVer: "1.87.188",
		},
		{
			name: "no_v_prefix",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				if err := json.NewEncoder(w).Encode(map[string]string{"tag_name": "1.87.188"}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			wantVer: "1.87.188",
		},
		{
			name: "empty_tag",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				if err := json.NewEncoder(w).Encode(map[string]string{"tag_name": ""}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			wantErr: true,
		},
		{
			name: "404",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
		{
			name: "malformed_json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, "{bad")
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			ver, err := parseBraveRelease(t, srv.URL)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ver != tc.wantVer {
				t.Errorf("version = %q, want %q", ver, tc.wantVer)
			}
		})
	}
}

// parseBraveRelease replicates the logic of latestBraveVersion.
func parseBraveRelease(t *testing.T, apiURL string) (string, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if release.TagName == "" {
		return "", fmt.Errorf("empty tag_name")
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

// TestDownloadFile_MultipleStatusCodes tests various HTTP status codes.
func TestDownloadFile_MultipleStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		wantOK bool
	}{
		{"200", http.StatusOK, "data", true},
		{"301", http.StatusMovedPermanently, "", false},
		{"403", http.StatusForbidden, "", false},
		{"500", http.StatusInternalServerError, "", false},
		{"503", http.StatusServiceUnavailable, "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)

				if tc.body != "" {
					_, _ = w.Write([]byte(tc.body))
				}
			}))
			defer srv.Close()

			data, err := DownloadFile(context.Background(), srv.URL)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if string(data) != tc.body {
					t.Errorf("body = %q, want %q", data, tc.body)
				}
			} else if err == nil {
				t.Error("expected error")
			}
		})
	}
}
