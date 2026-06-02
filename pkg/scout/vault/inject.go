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
// Call BEFORE navigating to the target origin. Cookies and headers take effect on
// the next navigation.
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
// returned buffer is owned by the profile; do not Close it directly — Close the
// Handle (or the Vault) instead.
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
