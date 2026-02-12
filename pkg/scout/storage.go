package scout

import (
	"encoding/json"
	"fmt"
	"os"
)

type storageType string

const (
	storageLocal   storageType = "localStorage"
	storageSession storageType = "sessionStorage"
)

type storageAPI struct {
	page *Page
	kind storageType
}

// Get returns the value for the given key, or empty string if not found.
func (s *storageAPI) Get(key string) (string, error) {
	res, err := s.page.Eval(fmt.Sprintf(`() => %s.getItem(%q) || ""`, s.kind, key))
	if err != nil {
		return "", fmt.Errorf("scout: %s get %q: %w", s.kind, key, err)
	}

	return res.String(), nil
}

// Set stores a key-value pair.
func (s *storageAPI) Set(key, value string) error {
	if _, err := s.page.Eval(fmt.Sprintf(`() => %s.setItem(%q, %q)`, s.kind, key, value)); err != nil {
		return fmt.Errorf("scout: %s set %q: %w", s.kind, key, err)
	}

	return nil
}

// Remove deletes the given key.
func (s *storageAPI) Remove(key string) error {
	if _, err := s.page.Eval(fmt.Sprintf(`() => %s.removeItem(%q)`, s.kind, key)); err != nil {
		return fmt.Errorf("scout: %s remove %q: %w", s.kind, key, err)
	}

	return nil
}

// Clear removes all keys from the storage.
func (s *storageAPI) Clear() error {
	if _, err := s.page.Eval(fmt.Sprintf(`() => %s.clear()`, s.kind)); err != nil {
		return fmt.Errorf("scout: %s clear: %w", s.kind, err)
	}

	return nil
}

// GetAll returns all key-value pairs in the storage.
func (s *storageAPI) GetAll() (map[string]string, error) {
	res, err := s.page.Eval(fmt.Sprintf(`() => {
		const m = {};
		for (let i = 0; i < %s.length; i++) {
			const k = %s.key(i);
			m[k] = %s.getItem(k);
		}
		return JSON.stringify(m);
	}`, s.kind, s.kind, s.kind))
	if err != nil {
		return nil, fmt.Errorf("scout: %s get all: %w", s.kind, err)
	}

	m := make(map[string]string)
	if err := json.Unmarshal([]byte(res.String()), &m); err != nil {
		return nil, fmt.Errorf("scout: %s get all unmarshal: %w", s.kind, err)
	}

	return m, nil
}

// Length returns the number of keys in the storage.
func (s *storageAPI) Length() (int, error) {
	res, err := s.page.Eval(fmt.Sprintf(`() => %s.length`, s.kind))
	if err != nil {
		return 0, fmt.Errorf("scout: %s length: %w", s.kind, err)
	}

	return res.Int(), nil
}

func (p *Page) localStorage() *storageAPI {
	return &storageAPI{page: p, kind: storageLocal}
}

func (p *Page) sessionStorage() *storageAPI {
	return &storageAPI{page: p, kind: storageSession}
}

// LocalStorageGet returns the value for the given key from localStorage.
func (p *Page) LocalStorageGet(key string) (string, error) {
	return p.localStorage().Get(key)
}

// LocalStorageSet stores a key-value pair in localStorage.
func (p *Page) LocalStorageSet(key, value string) error {
	return p.localStorage().Set(key, value)
}

// LocalStorageRemove deletes the given key from localStorage.
func (p *Page) LocalStorageRemove(key string) error {
	return p.localStorage().Remove(key)
}

// LocalStorageClear removes all keys from localStorage.
func (p *Page) LocalStorageClear() error {
	return p.localStorage().Clear()
}

// LocalStorageGetAll returns all key-value pairs from localStorage.
func (p *Page) LocalStorageGetAll() (map[string]string, error) {
	return p.localStorage().GetAll()
}

// LocalStorageLength returns the number of keys in localStorage.
func (p *Page) LocalStorageLength() (int, error) {
	return p.localStorage().Length()
}

// SessionStorageGet returns the value for the given key from sessionStorage.
func (p *Page) SessionStorageGet(key string) (string, error) {
	return p.sessionStorage().Get(key)
}

// SessionStorageSet stores a key-value pair in sessionStorage.
func (p *Page) SessionStorageSet(key, value string) error {
	return p.sessionStorage().Set(key, value)
}

// SessionStorageRemove deletes the given key from sessionStorage.
func (p *Page) SessionStorageRemove(key string) error {
	return p.sessionStorage().Remove(key)
}

// SessionStorageClear removes all keys from sessionStorage.
func (p *Page) SessionStorageClear() error {
	return p.sessionStorage().Clear()
}

// SessionStorageGetAll returns all key-value pairs from sessionStorage.
func (p *Page) SessionStorageGetAll() (map[string]string, error) {
	return p.sessionStorage().GetAll()
}

// SessionStorageLength returns the number of keys in sessionStorage.
func (p *Page) SessionStorageLength() (int, error) {
	return p.sessionStorage().Length()
}

// SessionState holds all restorable browser state for a page.
type SessionState struct {
	URL            string            `json:"url"`
	Cookies        []Cookie          `json:"cookies"`
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
}

// SaveSession captures the current URL, cookies, localStorage, and sessionStorage.
func (p *Page) SaveSession() (*SessionState, error) {
	pageURL, err := p.URL()
	if err != nil {
		return nil, fmt.Errorf("scout: save session url: %w", err)
	}

	cookies, err := p.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("scout: save session cookies: %w", err)
	}

	ls, err := p.LocalStorageGetAll()
	if err != nil {
		return nil, fmt.Errorf("scout: save session localStorage: %w", err)
	}

	ss, err := p.SessionStorageGetAll()
	if err != nil {
		return nil, fmt.Errorf("scout: save session sessionStorage: %w", err)
	}

	return &SessionState{
		URL:            pageURL,
		Cookies:        cookies,
		LocalStorage:   ls,
		SessionStorage: ss,
	}, nil
}

// LoadSession navigates to the saved URL and restores cookies, localStorage, and sessionStorage.
func (p *Page) LoadSession(state *SessionState) error {
	if err := p.Navigate(state.URL); err != nil {
		return fmt.Errorf("scout: load session navigate: %w", err)
	}

	if err := p.WaitLoad(); err != nil {
		return fmt.Errorf("scout: load session wait load: %w", err)
	}

	if len(state.Cookies) > 0 {
		if err := p.SetCookies(state.Cookies...); err != nil {
			return fmt.Errorf("scout: load session cookies: %w", err)
		}
	}

	for k, v := range state.LocalStorage {
		if err := p.LocalStorageSet(k, v); err != nil {
			return fmt.Errorf("scout: load session localStorage set %q: %w", k, err)
		}
	}

	for k, v := range state.SessionStorage {
		if err := p.SessionStorageSet(k, v); err != nil {
			return fmt.Errorf("scout: load session sessionStorage set %q: %w", k, err)
		}
	}

	return nil
}

// SaveSessionToFile marshals the session state to JSON and writes it to the given path.
func SaveSessionToFile(state *SessionState, path string) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: save session to file marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("scout: save session to file write: %w", err)
	}

	return nil
}

// LoadSessionFromFile reads JSON from the given path and unmarshals it into a SessionState.
func LoadSessionFromFile(path string) (*SessionState, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("scout: load session from file read: %w", err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("scout: load session from file unmarshal: %w", err)
	}

	return &state, nil
}
