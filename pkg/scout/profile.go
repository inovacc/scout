package scout

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout/scraper"
)

// UserProfile represents a portable browser identity that can be saved,
// loaded, and applied to browser sessions. It captures browser configuration,
// identity fingerprint data, cookies, storage, and custom headers.
type UserProfile struct {
	Version    int                             `json:"version"`
	Name       string                          `json:"name"`
	CreatedAt  time.Time                       `json:"created_at"`
	UpdatedAt  time.Time                       `json:"updated_at"`
	Browser    ProfileBrowser                  `json:"browser"`
	Identity   ProfileIdentity                 `json:"identity"`
	Cookies    []Cookie                        `json:"cookies"`
	Storage    map[string]ProfileOriginStorage `json:"storage,omitempty"`
	Headers    map[string]string               `json:"headers,omitempty"`
	Extensions []string                        `json:"extensions,omitempty"`
	Proxy      string                          `json:"proxy,omitempty"`
	Notes      string                          `json:"notes,omitempty"`
}

// ProfileBrowser holds browser type and launch configuration.
type ProfileBrowser struct {
	Type     string `json:"type,omitempty"`
	ExecPath string `json:"exec_path,omitempty"`
	WindowW  int    `json:"window_w,omitempty"`
	WindowH  int    `json:"window_h,omitempty"`
	Platform string `json:"platform,omitempty"`
	Arch     string `json:"arch,omitempty"`
}

// ProfileIdentity holds browser fingerprint identity fields.
type ProfileIdentity struct {
	UserAgent string `json:"user_agent,omitempty"`
	Language  string `json:"language,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
	Locale    string `json:"locale,omitempty"`
}

// ProfileOriginStorage holds per-origin localStorage and sessionStorage data.
type ProfileOriginStorage struct {
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
}

// ProfileOption configures profile capture behavior.
type ProfileOption func(*profileConfig)

type profileConfig struct {
	name string
}

// WithProfileName sets the profile name.
func WithProfileName(name string) ProfileOption {
	return func(c *profileConfig) { c.name = name }
}

// CaptureProfile snapshots the current page's browser state into a UserProfile.
func CaptureProfile(page *Page, opts ...ProfileOption) (*UserProfile, error) {
	if page == nil || page.page == nil {
		return nil, fmt.Errorf("scout: profile: capture: nil page")
	}

	cfg := &profileConfig{}
	for _, fn := range opts {
		fn(cfg)
	}

	now := time.Now()
	p := &UserProfile{
		Version:   1,
		Name:      cfg.name,
		CreatedAt: now,
		UpdatedAt: now,
		Browser: ProfileBrowser{
			Platform: runtime.GOOS,
			Arch:     runtime.GOARCH,
		},
	}

	// Browser version.
	if page.browser != nil {
		if v, err := page.browser.Version(); err == nil {
			p.Browser.Type = v
		}
		if page.browser.opts != nil {
			p.Browser.WindowW = page.browser.opts.windowW
			p.Browser.WindowH = page.browser.opts.windowH
			if page.browser.opts.browserType != "" {
				p.Browser.Type = string(page.browser.opts.browserType)
			}
		}
	}

	// Identity via JS.
	if res, err := page.Eval(`() => navigator.userAgent`); err == nil {
		p.Identity.UserAgent = res.String()
	}

	if res, err := page.Eval(`() => navigator.language`); err == nil {
		p.Identity.Language = res.String()
	}

	if res, err := page.Eval(`() => Intl.DateTimeFormat().resolvedOptions().timeZone`); err == nil {
		p.Identity.Timezone = res.String()
	}

	if res, err := page.Eval(`() => Intl.DateTimeFormat().resolvedOptions().locale`); err == nil {
		p.Identity.Locale = res.String()
	}

	// Cookies.
	if cookies, err := page.GetCookies(); err == nil {
		p.Cookies = cookies
	}

	// Storage for current origin.
	pageURL, _ := page.URL()
	if pageURL != "" {
		origin := originFromURL(pageURL)
		if origin != "" {
			os := ProfileOriginStorage{}

			if ls, err := page.LocalStorageGetAll(); err == nil && len(ls) > 0 {
				os.LocalStorage = ls
			}

			if ss, err := page.SessionStorageGetAll(); err == nil && len(ss) > 0 {
				os.SessionStorage = ss
			}

			if len(os.LocalStorage) > 0 || len(os.SessionStorage) > 0 {
				p.Storage = map[string]ProfileOriginStorage{origin: os}
			}
		}
	}

	// Headers from browser options.
	if page.browser != nil && page.browser.opts != nil {
		if page.browser.opts.proxy != "" {
			p.Proxy = page.browser.opts.proxy
		}
	}

	return p, nil
}

// SaveProfile writes a UserProfile to a JSON file with 0600 permissions.
func SaveProfile(p *UserProfile, path string) error {
	if p == nil {
		return fmt.Errorf("scout: profile: save: nil profile")
	}

	p.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: profile: save: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("scout: profile: save: write: %w", err)
	}

	return nil
}

// LoadProfile reads a UserProfile from a JSON file.
func LoadProfile(path string) (*UserProfile, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("scout: profile: load: read: %w", err)
	}

	var p UserProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("scout: profile: load: unmarshal: %w", err)
	}

	return &p, nil
}

// WithProfile returns an Option that loads a profile from disk and applies
// browser-level settings (user agent, proxy, window size) at launch time.
// Page-level settings (cookies, storage, headers) must be applied after page
// creation via Page.ApplyProfile.
func WithProfile(path string) Option {
	return func(o *options) {
		p, err := LoadProfile(path)
		if err != nil {
			return
		}
		applyProfileToOptions(p, o)
	}
}

// WithProfileData applies an in-memory profile to browser options.
func WithProfileData(p *UserProfile) Option {
	return func(o *options) {
		if p == nil {
			return
		}
		applyProfileToOptions(p, o)
	}
}

func applyProfileToOptions(p *UserProfile, o *options) {
	if p.Identity.UserAgent != "" {
		o.userAgent = p.Identity.UserAgent
	}

	if p.Proxy != "" {
		o.proxy = p.Proxy
	}

	if p.Browser.WindowW > 0 && p.Browser.WindowH > 0 {
		o.windowW = p.Browser.WindowW
		o.windowH = p.Browser.WindowH
	}

	if p.Browser.Type != "" {
		o.browserType = BrowserType(p.Browser.Type)
	}

	if p.Browser.ExecPath != "" {
		o.execPath = p.Browser.ExecPath
	}

	if len(p.Extensions) > 0 {
		resolved := ResolveExtensions(p)
		o.extensions = append(o.extensions, resolved...)
	}

	// Store profile on options for later use by NewPage / ApplyProfile.
	o.profile = p
}

// ResolveExtensions resolves extension entries in a profile to valid filesystem paths.
// Each entry is checked as-is first (absolute path). If the path does not exist,
// it is treated as an extension ID and looked up in ~/.scout/extensions/<id>/.
// Missing extensions produce a warning log but do not cause errors.
func ResolveExtensions(p *UserProfile) []string {
	return ResolveExtensionsWithBase(p, "")
}

// ResolveExtensionsWithBase resolves extensions using a custom base directory
// instead of the default ~/.scout/extensions/. If baseDir is empty, the default
// is used. This variant exists for testing.
func ResolveExtensionsWithBase(p *UserProfile, baseDir string) []string {
	if p == nil || len(p.Extensions) == 0 {
		return nil
	}

	var resolved []string

	for _, ext := range p.Extensions {
		// Check if the path exists as-is.
		if info, err := os.Stat(ext); err == nil && info.IsDir() {
			resolved = append(resolved, ext)
			continue
		}

		// Treat as extension ID — look up in base dir.
		var dir string
		if baseDir != "" {
			dir = filepath.Join(baseDir, ext)
		} else {
			extDir, err := ExtensionDir()
			if err != nil {
				slog.Warn("scout: profile: resolve extensions: cannot determine extension dir", "error", err)
				continue
			}
			dir = filepath.Join(extDir, ext)
		}

		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			resolved = append(resolved, dir)
			continue
		}

		slog.Warn("scout: profile: extension not found, skipping", "extension", ext)
	}

	return resolved
}

// ApplyProfile restores page-level state from a UserProfile: cookies, storage,
// and headers. Call this after page creation and navigation to the target origin.
func (p *Page) ApplyProfile(prof *UserProfile) error {
	if p == nil || p.page == nil {
		return fmt.Errorf("scout: profile: apply: nil page")
	}

	if prof == nil {
		return fmt.Errorf("scout: profile: apply: nil profile")
	}

	// Set cookies.
	if len(prof.Cookies) > 0 {
		if err := p.SetCookies(prof.Cookies...); err != nil {
			return fmt.Errorf("scout: profile: apply cookies: %w", err)
		}
	}

	// Set headers.
	if len(prof.Headers) > 0 {
		if _, err := p.SetHeaders(prof.Headers); err != nil {
			return fmt.Errorf("scout: profile: apply headers: %w", err)
		}
	}

	// Inject storage for the current origin.
	pageURL, _ := p.URL()
	origin := originFromURL(pageURL)

	if origin != "" {
		if storage, ok := prof.Storage[origin]; ok {
			for k, v := range storage.LocalStorage {
				if err := p.LocalStorageSet(k, v); err != nil {
					return fmt.Errorf("scout: profile: apply localStorage %q: %w", k, err)
				}
			}

			for k, v := range storage.SessionStorage {
				if err := p.SessionStorageSet(k, v); err != nil {
					return fmt.Errorf("scout: profile: apply sessionStorage %q: %w", k, err)
				}
			}
		}
	}

	return nil
}

// SaveProfileEncrypted writes an encrypted profile using AES-256-GCM + Argon2id.
func SaveProfileEncrypted(p *UserProfile, path, passphrase string) error {
	if p == nil {
		return fmt.Errorf("scout: profile: save encrypted: nil profile")
	}

	p.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: profile: save encrypted: marshal: %w", err)
	}

	encrypted, err := scraper.EncryptData(data, passphrase)
	if err != nil {
		return fmt.Errorf("scout: profile: save encrypted: %w", err)
	}

	if err := os.WriteFile(path, encrypted, 0o600); err != nil {
		return fmt.Errorf("scout: profile: save encrypted: write: %w", err)
	}

	return nil
}

// LoadProfileEncrypted reads and decrypts a profile.
func LoadProfileEncrypted(path, passphrase string) (*UserProfile, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("scout: profile: load encrypted: read: %w", err)
	}

	decrypted, err := scraper.DecryptData(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("scout: profile: load encrypted: %w", err)
	}

	var p UserProfile
	if err := json.Unmarshal(decrypted, &p); err != nil {
		return nil, fmt.Errorf("scout: profile: load encrypted: unmarshal: %w", err)
	}

	return &p, nil
}

// MergeProfiles merges two profiles. Overlay values win on conflict.
// Cookies and storage are merged (overlay additions/updates win, base-only entries kept).
func MergeProfiles(base, overlay *UserProfile) *UserProfile {
	merged := &UserProfile{
		Version:   base.Version,
		CreatedAt: base.CreatedAt,
		UpdatedAt: base.UpdatedAt,
	}

	// Use earliest CreatedAt.
	if overlay.CreatedAt.Before(base.CreatedAt) {
		merged.CreatedAt = overlay.CreatedAt
	}

	// Use latest UpdatedAt.
	if overlay.UpdatedAt.After(base.UpdatedAt) {
		merged.UpdatedAt = overlay.UpdatedAt
	}

	// Scalar fields: overlay wins if non-empty.
	merged.Name = base.Name
	if overlay.Name != "" {
		merged.Name = overlay.Name
	}

	merged.Notes = base.Notes
	if overlay.Notes != "" {
		merged.Notes = overlay.Notes
	}

	merged.Proxy = base.Proxy
	if overlay.Proxy != "" {
		merged.Proxy = overlay.Proxy
	}

	// Identity: overlay wins if non-empty.
	merged.Identity = base.Identity
	if overlay.Identity.UserAgent != "" {
		merged.Identity.UserAgent = overlay.Identity.UserAgent
	}
	if overlay.Identity.Language != "" {
		merged.Identity.Language = overlay.Identity.Language
	}
	if overlay.Identity.Timezone != "" {
		merged.Identity.Timezone = overlay.Identity.Timezone
	}
	if overlay.Identity.Locale != "" {
		merged.Identity.Locale = overlay.Identity.Locale
	}

	// Browser: overlay wins if any field set.
	merged.Browser = base.Browser
	if overlay.Browser.Type != "" {
		merged.Browser.Type = overlay.Browser.Type
	}
	if overlay.Browser.ExecPath != "" {
		merged.Browser.ExecPath = overlay.Browser.ExecPath
	}
	if overlay.Browser.WindowW > 0 {
		merged.Browser.WindowW = overlay.Browser.WindowW
	}
	if overlay.Browser.WindowH > 0 {
		merged.Browser.WindowH = overlay.Browser.WindowH
	}
	if overlay.Browser.Platform != "" {
		merged.Browser.Platform = overlay.Browser.Platform
	}
	if overlay.Browser.Arch != "" {
		merged.Browser.Arch = overlay.Browser.Arch
	}

	// Cookies: merge by domain+name+path key (overlay wins on conflict).
	type cookieKey struct {
		Domain, Name, Path string
	}
	cookieMap := make(map[cookieKey]Cookie)
	for _, c := range base.Cookies {
		cookieMap[cookieKey{c.Domain, c.Name, c.Path}] = c
	}
	for _, c := range overlay.Cookies {
		cookieMap[cookieKey{c.Domain, c.Name, c.Path}] = c
	}
	merged.Cookies = make([]Cookie, 0, len(cookieMap))
	for _, c := range cookieMap {
		merged.Cookies = append(merged.Cookies, c)
	}

	// Storage: merge per-origin (overlay origins win, base-only origins kept).
	if len(base.Storage) > 0 || len(overlay.Storage) > 0 {
		merged.Storage = make(map[string]ProfileOriginStorage)
		for origin, s := range base.Storage {
			merged.Storage[origin] = s
		}
		for origin, s := range overlay.Storage {
			merged.Storage[origin] = s
		}
	}

	// Headers: merge maps (overlay wins).
	if len(base.Headers) > 0 || len(overlay.Headers) > 0 {
		merged.Headers = make(map[string]string)
		for k, v := range base.Headers {
			merged.Headers[k] = v
		}
		for k, v := range overlay.Headers {
			merged.Headers[k] = v
		}
	}

	// Extensions: union (deduplicated).
	extSet := make(map[string]struct{})
	for _, e := range base.Extensions {
		extSet[e] = struct{}{}
	}
	for _, e := range overlay.Extensions {
		extSet[e] = struct{}{}
	}
	if len(extSet) > 0 {
		merged.Extensions = make([]string, 0, len(extSet))
		for e := range extSet {
			merged.Extensions = append(merged.Extensions, e)
		}
	}

	return merged
}

// ProfileDiff summarizes the differences between two profiles.
type ProfileDiff struct {
	NameChanged           bool `json:"name_changed,omitempty"`
	IdentityChanged       bool `json:"identity_changed,omitempty"`
	BrowserChanged        bool `json:"browser_changed,omitempty"`
	CookiesAdded          int  `json:"cookies_added,omitempty"`
	CookiesRemoved        int  `json:"cookies_removed,omitempty"`
	CookiesModified       int  `json:"cookies_modified,omitempty"`
	StorageOriginsAdded   int  `json:"storage_origins_added,omitempty"`
	StorageOriginsRemoved int  `json:"storage_origins_removed,omitempty"`
	HeadersChanged        int  `json:"headers_changed,omitempty"`
	ExtensionsAdded       int  `json:"extensions_added,omitempty"`
	ExtensionsRemoved     int  `json:"extensions_removed,omitempty"`
}

// DiffProfiles compares two profiles and returns a summary of differences.
func DiffProfiles(a, b *UserProfile) ProfileDiff {
	var d ProfileDiff

	d.NameChanged = a.Name != b.Name
	d.IdentityChanged = a.Identity != b.Identity
	d.BrowserChanged = a.Browser != b.Browser

	// Cookies diff by domain+name+path key.
	type cookieKey struct {
		Domain, Name, Path string
	}
	aCookies := make(map[cookieKey]Cookie)
	for _, c := range a.Cookies {
		aCookies[cookieKey{c.Domain, c.Name, c.Path}] = c
	}
	bCookies := make(map[cookieKey]Cookie)
	for _, c := range b.Cookies {
		bCookies[cookieKey{c.Domain, c.Name, c.Path}] = c
	}
	for k, bc := range bCookies {
		if ac, ok := aCookies[k]; !ok {
			d.CookiesAdded++
		} else if ac.Value != bc.Value {
			d.CookiesModified++
		}
	}
	for k := range aCookies {
		if _, ok := bCookies[k]; !ok {
			d.CookiesRemoved++
		}
	}

	// Storage origins.
	for origin := range b.Storage {
		if _, ok := a.Storage[origin]; !ok {
			d.StorageOriginsAdded++
		}
	}
	for origin := range a.Storage {
		if _, ok := b.Storage[origin]; !ok {
			d.StorageOriginsRemoved++
		}
	}

	// Headers: count keys that differ or are added/removed.
	allHeaders := make(map[string]struct{})
	for k := range a.Headers {
		allHeaders[k] = struct{}{}
	}
	for k := range b.Headers {
		allHeaders[k] = struct{}{}
	}
	for k := range allHeaders {
		va, oka := a.Headers[k]
		vb, okb := b.Headers[k]
		if oka != okb || va != vb {
			d.HeadersChanged++
		}
	}

	// Extensions.
	aExts := make(map[string]struct{})
	for _, e := range a.Extensions {
		aExts[e] = struct{}{}
	}
	bExts := make(map[string]struct{})
	for _, e := range b.Extensions {
		bExts[e] = struct{}{}
	}
	for e := range bExts {
		if _, ok := aExts[e]; !ok {
			d.ExtensionsAdded++
		}
	}
	for e := range aExts {
		if _, ok := bExts[e]; !ok {
			d.ExtensionsRemoved++
		}
	}

	return d
}

// Validate checks a profile for required fields and consistency.
func (p *UserProfile) Validate() error {
	var errs []string

	if p.Version <= 0 {
		errs = append(errs, "version must be > 0")
	}

	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, "name is required")
	}

	for i, c := range p.Cookies {
		if c.Domain == "" {
			errs = append(errs, fmt.Sprintf("cookie[%d] %q: domain is required", i, c.Name))
		}
	}

	for origin := range p.Storage {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" {
			errs = append(errs, fmt.Sprintf("storage origin %q: invalid URL", origin))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("scout: profile: validate: %w", errors.New(strings.Join(errs, "; ")))
	}

	return nil
}

// originFromURL extracts the scheme+host origin from a URL string.
func originFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}

	return u.Scheme + "://" + u.Host
}
