package vault

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// FromUserProfile reads a .scoutprofile file and extracts only its secret-bearing
// fields (cookies, per-origin storage, auth headers) into a SecretProfileInput.
// Non-secret identity fields (UA/lang/tz/locale/proxy/extensions) are left in the
// UserProfile.
func FromUserProfile(path string) (SecretProfileInput, error) {
	up, err := scout.LoadProfile(path)
	if err != nil {
		return SecretProfileInput{}, fmt.Errorf("scout: vault: load profile: %w", err)
	}
	in := SecretProfileInput{Name: up.Name, Cookies: up.Cookies}
	if len(up.Headers) > 0 {
		in.Headers = make(map[string][]byte, len(up.Headers))
		for k, val := range up.Headers {
			in.Headers[k] = []byte(val)
		}
	}
	if len(up.Storage) > 0 {
		in.Storage = make(map[string]OriginStore, len(up.Storage))
		for origin, os := range up.Storage {
			in.Storage[origin] = OriginStore{LocalStorage: os.LocalStorage, SessionStorage: os.SessionStorage}
		}
	}
	return in, nil
}
