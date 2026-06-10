package vault

import (
	"fmt"
	"net/url"

	"github.com/inovacc/scout/pkg/scout"
)

// CaptureFromPage snapshots a live page's secret-bearing browser state — cookies
// plus the current origin's localStorage/sessionStorage — into a SecretProfileInput
// named name. Auth headers are not captured (script cannot read outbound request
// headers). Local sessions only; the page must already be navigated to (and
// authenticated at) the target origin.
func CaptureFromPage(page *scout.Page, name string) (SecretProfileInput, error) {
	if page == nil {
		return SecretProfileInput{}, fmt.Errorf("scout: vault: capture: nil page")
	}

	in := SecretProfileInput{Name: name}

	cookies, err := page.GetCookies()
	if err != nil {
		return SecretProfileInput{}, fmt.Errorf("scout: vault: capture cookies: %w", err)
	}
	in.Cookies = cookies

	pageURL, _ := page.URL()
	if origin := originFrom(pageURL); origin != "" {
		store := OriginStore{}
		if ls, err := page.LocalStorageGetAll(); err == nil && len(ls) > 0 {
			store.LocalStorage = ls
		}
		if ss, err := page.SessionStorageGetAll(); err == nil && len(ss) > 0 {
			store.SessionStorage = ss
		}
		if len(store.LocalStorage) > 0 || len(store.SessionStorage) > 0 {
			in.Storage = map[string]OriginStore{origin: store}
		}
	}

	return in, nil
}

// originFrom extracts the scheme+host origin from a URL string ("" if unparseable).
func originFrom(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
