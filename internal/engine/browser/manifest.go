package browser

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

//go:embed browser.json
var manifestJSON []byte

// Manifest represents the browser.json configuration.
type Manifest struct {
	Chromium ChromiumConfig `json:"chromium"`
	Chrome   BrowserDef     `json:"chrome"`
	Brave    BrowserDef     `json:"brave"`
	Edge     BrowserDef     `json:"edge"`
}

// ChromiumConfig holds Chromium download configuration.
type ChromiumConfig struct {
	Revision  RevisionConfig            `json:"revision"`
	Hosts     map[string]HostConfig     `json:"hosts"`
	Platforms map[string]PlatformConfig `json:"platforms"`
}

// RevisionConfig holds revision numbers and latest-check URLs.
type RevisionConfig struct {
	Default   int               `json:"default"`
	LatestAPI map[string]string `json:"latest_api"`
}

// HostConfig describes a download host template.
type HostConfig struct {
	Base    string `json:"base"`
	Pattern string `json:"pattern"`
}

// PlatformConfig describes per-platform download details.
type PlatformConfig struct {
	Prefix string   `json:"prefix"`
	Zip    string   `json:"zip"`
	Binary string   `json:"binary"`
	URLs   []string `json:"urls"`
}

// BrowserDef is a generic browser definition for chrome, brave, and edge.
type BrowserDef struct {
	Description string                        `json:"description"`
	API         map[string]string             `json:"api"`
	ReleaseBase string                        `json:"release_base"`
	Platforms   map[string]BrowserPlatformDef `json:"platforms"`
}

// BrowserPlatformDef describes per-platform download details for a browser.
type BrowserPlatformDef struct {
	PlatformID string   `json:"platform_id"` // CfT platform identifier (e.g. "win64")
	Zip        string   `json:"zip"`
	Binary     string   `json:"binary"`
	URLs       []string `json:"urls"`
}

var (
	manifestOnce sync.Once
	manifestInst *Manifest
)

// LoadManifest parses browser.json once and returns the cached result.
func LoadManifest() *Manifest {
	manifestOnce.Do(func() {
		var m Manifest
		if err := json.Unmarshal(manifestJSON, &m); err != nil {
			panic(fmt.Sprintf("browser: parse browser.json: %v", err))
		}

		manifestInst = &m
	})

	return manifestInst
}

// DefaultRevision returns the default Chromium snapshot revision.
func (m *Manifest) DefaultRevision() int {
	return m.Chromium.Revision.Default
}

// PlatformKey returns the current GOOS_GOARCH key.
func PlatformKey() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}

// Platform returns the PlatformConfig for the current GOOS_GOARCH, or nil.
func (m *Manifest) Platform() *PlatformConfig {
	p, ok := m.Chromium.Platforms[PlatformKey()]
	if !ok {
		return nil
	}

	return &p
}

// HostURLs returns Host functions generated from the platform's URL templates.
func (m *Manifest) HostURLs(revision int) []HostFunc {
	p := m.Platform()
	if p == nil {
		return nil
	}

	rev := fmt.Sprintf("%d", revision)

	var hosts []HostFunc

	for _, tmpl := range p.URLs {
		t := tmpl // capture

		hosts = append(hosts, func(_ int) string {
			return strings.ReplaceAll(t, "{revision}", rev)
		})
	}

	return hosts
}

// LatestAPI returns the LAST_CHANGE URL for the current platform, or empty string.
func (m *Manifest) LatestAPI() string {
	return m.Chromium.Revision.LatestAPI[PlatformKey()]
}

// BrowserAPI returns an API URL from a browser definition by key.
func (d *BrowserDef) BrowserAPI(key string) string {
	if d.API == nil {
		return ""
	}

	return d.API[key]
}

// BrowserPlatform returns the BrowserPlatformDef for the current GOOS_GOARCH, or nil.
func (d *BrowserDef) BrowserPlatform() *BrowserPlatformDef {
	p, ok := d.Platforms[PlatformKey()]
	if !ok {
		return nil
	}

	return &p
}

// BinaryPath returns the binary path for the current platform, or fallback.
func (d *BrowserDef) BinaryPath(fallback string) string {
	p := d.BrowserPlatform()
	if p == nil || p.Binary == "" {
		return fallback
	}

	return p.Binary
}

// ZipName returns the zip filename for the current platform, or empty.
func (d *BrowserDef) ZipName(version string) string {
	p := d.BrowserPlatform()
	if p == nil {
		return ""
	}

	return strings.ReplaceAll(p.Zip, "{version}", version)
}

// DownloadURL returns the first download URL for the current platform with
// {version} placeholders replaced. Returns empty string if unavailable.
func (d *BrowserDef) DownloadURL(version string) string {
	p := d.BrowserPlatform()
	if p == nil || len(p.URLs) == 0 {
		return ""
	}

	return strings.ReplaceAll(p.URLs[0], "{version}", version)
}

// HostFunc formats a revision number to a downloadable URL for the browser.
type HostFunc func(revision int) string

// HostConf returns the platform config from the manifest for the current OS/arch.
func HostConf() *PlatformConfig {
	return LoadManifest().Platform()
}
