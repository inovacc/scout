package b3pipe

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/vault"
)

// FetchResult holds the HTTP status and raw response body from an in-page fetch.
type FetchResult struct {
	Status int    `json:"status"`
	Body   []byte `json:"-"`
}

// buildFetchJS returns an async IIFE that fetches endpoint and resolves to
// {status, body}. In bearer mode it reads the JWT from localStorage and sets
// Authorization: Bearer <tok>; in cookie mode it sends credentials:'include'.
func buildFetchJS(endpoint string, auth AuthConfig) string {
	var headers string
	if auth.Mode == "bearer" {
		headers = fmt.Sprintf(
			"const tok = localStorage.getItem('%s'); "+
				"const headers = tok ? {'Authorization': 'Bearer ' + tok} : {};",
			auth.TokenStorageKey)
	} else {
		headers = "const headers = {};"
	}
	return fmt.Sprintf(`async () => {
  %s
  const r = await fetch(%q, { headers, credentials: 'include' });
  const body = await r.text();
  return { status: r.status, body };
}`, headers, endpoint)
}

// FetchSection runs the section's fetch JS in the page and returns status+body.
func FetchSection(page *scout.Page, s Section, auth AuthConfig) (FetchResult, error) {
	res, err := page.Eval(buildFetchJS(s.Endpoint, auth))
	if err != nil {
		return FetchResult{}, fmt.Errorf("b3: fetch %s: %w", s.ID, err)
	}
	var payload struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal(res.JSON(), &payload); err != nil {
		return FetchResult{}, fmt.Errorf("b3: decode %s: %w", s.ID, err)
	}
	return FetchResult{Status: payload.Status, Body: []byte(payload.Body)}, nil
}

// OpenAuthedPage opens a headless authenticated page for the given vault profile.
// Sequence: open vault → use profile → new browser → new page (about:blank) →
// ApplyToPage (cookies+headers, pre-nav) → Navigate → WaitLoad →
// ApplyStorageToPage (localStorage/sessionStorage, on origin) →
// Navigate again → WaitLoad (reload so the SPA boots with seeded storage).
// Caller is responsible for closing browser and handle.
func OpenAuthedPage(profileID string, pass []byte, baseURL string) (*scout.Browser, *scout.Page, *vault.Handle, error) {
	v, err := vault.Open(pass)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("b3: vault open: %w", err)
	}
	h, err := v.Use(profileID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("b3: vault use: %w", err)
	}
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		_ = h.Close()
		return nil, nil, nil, fmt.Errorf("b3: browser: %w", err)
	}
	page, err := b.NewPage("about:blank")
	if err != nil {
		b.Close()
		_ = h.Close()
		return nil, nil, nil, fmt.Errorf("b3: new page: %w", err)
	}
	if err := h.ApplyToPage(page); err != nil {
		b.Close()
		_ = h.Close()
		return nil, nil, nil, fmt.Errorf("b3: apply cookies: %w", err)
	}
	if err := page.Navigate(baseURL); err != nil {
		b.Close()
		_ = h.Close()
		return nil, nil, nil, fmt.Errorf("b3: navigate: %w", err)
	}
	_ = page.WaitLoad()
	if err := h.ApplyStorageToPage(page); err != nil {
		b.Close()
		_ = h.Close()
		return nil, nil, nil, fmt.Errorf("b3: apply storage: %w", err)
	}
	if err := page.Navigate(baseURL); err != nil {
		b.Close()
		_ = h.Close()
		return nil, nil, nil, fmt.Errorf("b3: re-navigate: %w", err)
	}
	_ = page.WaitLoad()
	return b, page, h, nil
}
