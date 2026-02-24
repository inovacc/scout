package scout

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"time"
)

// UserProfile represents a portable browser identity that can be saved,
// loaded, and applied to browser sessions. It captures browser configuration,
// identity fingerprint data, cookies, storage, and custom headers.
type UserProfile struct {
	Version    int                              `json:"version"`
	Name       string                           `json:"name"`
	CreatedAt  time.Time                        `json:"created_at"`
	UpdatedAt  time.Time                        `json:"updated_at"`
	Browser    ProfileBrowser                   `json:"browser"`
	Identity   ProfileIdentity                  `json:"identity"`
	Cookies    []Cookie                         `json:"cookies"`
	Storage    map[string]ProfileOriginStorage  `json:"storage,omitempty"`
	Headers    map[string]string                `json:"headers,omitempty"`
	Extensions []string                         `json:"extensions,omitempty"`
	Proxy      string                           `json:"proxy,omitempty"`
	Notes      string                           `json:"notes,omitempty"`
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
		o.extensions = append(o.extensions, p.Extensions...)
	}

	// Store profile on options for later use by NewPage / ApplyProfile.
	o.profile = p
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

// originFromURL extracts the scheme+host origin from a URL string.
func originFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}

	return u.Scheme + "://" + u.Host
}
