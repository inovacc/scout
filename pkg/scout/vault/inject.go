package vault

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// Handle is an operational view over one profile: it injects browser-bound
// secrets into a live page and yields scout-internal secrets as buffers.
type Handle struct {
	profile *SecretProfile
}

// ApplyToPage injects the profile's cookies and auth headers into page via CDP.
// Call BEFORE navigating to the target origin; cookies and headers take effect on
// the next navigation. Web storage (profile.Storage) is NOT seeded here — that is
// origin-specific and happens post-navigation. On error the page may be partially
// mutated (e.g. cookies set but headers not); the caller should discard it.
func (h *Handle) ApplyToPage(page *scout.Page) error {
	if len(h.profile.Cookies) > 0 {
		if err := page.SetCookies(h.profile.Cookies...); err != nil {
			return fmt.Errorf("scout: vault: inject cookies: %w", err)
		}
	}
	if len(h.profile.Headers) > 0 {
		hdr := make(map[string]string, len(h.profile.Headers))
		for k, lb := range h.profile.Headers {
			hdr[k] = string(lb.Bytes()) // vault:allow-string — CDP requires string headers; map is discarded after the call
		}
		if _, err := page.SetHeaders(hdr); err != nil {
			return fmt.Errorf("scout: vault: inject headers: %w", err)
		}
	}
	return nil
}

// Secret returns the named scout-internal secret as a zeroable buffer. The
// returned buffer is owned by the profile and is valid only until Handle.Close()
// (or Vault.Close()); do not Close it directly and do not retain it past then.
func (h *Handle) Secret(name string) (*LockedBuffer, error) {
	lb, ok := h.profile.Secrets[name]
	if !ok {
		return nil, fmt.Errorf("scout: vault: secret %q not found", name)
	}
	return lb, nil
}

// Close zeros the underlying profile's secret buffers.
func (h *Handle) Close() error {
	h.profile.Close()
	return nil
}

// ApplyStorageToPage seeds the current origin's localStorage and sessionStorage
// from the profile. Call AFTER navigating to the target origin (cookies and
// headers are applied pre-navigation via ApplyToPage). Storage for origins other
// than the page's current origin is ignored. No-op when the profile has no storage.
func (h *Handle) ApplyStorageToPage(page *scout.Page) error {
	if len(h.profile.Storage) == 0 {
		return nil
	}
	pageURL, _ := page.URL()
	origin := originFrom(pageURL)
	if origin == "" {
		return nil
	}
	store, ok := h.profile.Storage[origin]
	if !ok {
		return nil
	}
	for k, v := range store.LocalStorage {
		if err := page.LocalStorageSet(k, v); err != nil {
			return fmt.Errorf("scout: vault: inject localStorage %q: %w", k, err)
		}
	}
	for k, v := range store.SessionStorage {
		if err := page.SessionStorageSet(k, v); err != nil {
			return fmt.Errorf("scout: vault: inject sessionStorage %q: %w", k, err)
		}
	}
	return nil
}
