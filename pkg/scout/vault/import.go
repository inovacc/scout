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
	// The vault is the sanctioned migration target for these deprecated, secret-bearing fields.
	//nolint:staticcheck // SA1019: the importer must read the deprecated UserProfile secret fields
	cookies, headers, storage := up.Cookies, up.Headers, up.Storage

	in := SecretProfileInput{Name: up.Name, Cookies: cookies}
	if len(headers) > 0 {
		in.Headers = make(map[string][]byte, len(headers))
		for k, val := range headers {
			in.Headers[k] = []byte(val)
		}
	}
	if len(storage) > 0 {
		in.Storage = make(map[string]OriginStore, len(storage))
		for origin, os := range storage {
			in.Storage[origin] = OriginStore{LocalStorage: os.LocalStorage, SessionStorage: os.SessionStorage}
		}
	}
	return in, nil
}
