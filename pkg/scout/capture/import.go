package capture

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout/vault"
)

// ImportReport summarizes a drain.
type ImportReport struct {
	Imported    int
	Quarantined int
	Sites       []string
}

// ImportSpool decrypts every spool file, upserts it into the per-site vault
// profile, secure-deletes the file on success, and quarantines undecryptable ones.
func ImportSpool(v *vault.Vault, spoolDir string, pub, priv *[32]byte) (ImportReport, error) {
	files, err := ListSpool(spoolDir)
	if err != nil {
		return ImportReport{}, err
	}
	var rep ImportReport
	seen := map[string]bool{}
	for _, f := range files {
		plain, ok := OpenSpoolFile(f, pub, priv)
		if !ok {
			if qerr := Quarantine(f); qerr != nil {
				return rep, qerr
			}
			rep.Quarantined++
			continue
		}
		var m Msg
		if jerr := json.Unmarshal(plain, &m); jerr != nil || Validate(m) != nil {
			if qerr := Quarantine(f); qerr != nil {
				return rep, qerr
			}
			rep.Quarantined++
			continue
		}
		if uerr := upsertSite(v, m); uerr != nil {
			return rep, uerr
		}
		if derr := SecureDelete(f); derr != nil {
			return rep, derr
		}
		rep.Imported++
		if !seen[m.Site] {
			seen[m.Site] = true
			rep.Sites = append(rep.Sites, m.Site)
		}
	}
	return rep, nil
}

// upsertSite merges one capture message into the vault profile named m.Site.
func upsertSite(v *vault.Vault, m Msg) error {
	id := ""
	for _, meta := range v.List() {
		if meta.Name == m.Site {
			id = meta.ID
		}
	}

	in := vault.SecretProfileInput{ID: id, Name: m.Site, Secrets: map[string][]byte{}}

	// Carry forward existing secrets so a later login doesn't drop earlier ones.
	if id != "" {
		if sp, err := v.Get(id); err == nil {
			for k, lb := range sp.Secrets {
				in.Secrets[k] = append([]byte(nil), lb.Bytes()...)
			}
			in.Cookies = sp.Cookies
			in.Storage = sp.Storage
			sp.Close()
		}
	}

	switch m.Type {
	case "capture_session":
		in.Cookies = toVaultCookies(m.Cookies)
		in.Storage = toVaultStorage(m.Storage)
	case "capture_login":
		in.Secrets["login:"+m.Username] = []byte(m.Password)
	}

	if _, err := v.Set(in); err != nil {
		return fmt.Errorf("scout: capture: upsert %q: %w", m.Site, err)
	}
	return nil
}

func toVaultCookies(cs []WireCookie) []vault.Cookie {
	out := make([]vault.Cookie, 0, len(cs))
	for _, c := range cs {
		out = append(out, vault.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly})
	}
	return out
}

func toVaultStorage(s map[string]WireOriginStorage) map[string]vault.OriginStore {
	if len(s) == 0 {
		return nil
	}
	out := make(map[string]vault.OriginStore, len(s))
	for origin, st := range s {
		out[origin] = vault.OriginStore{LocalStorage: st.Local, SessionStorage: st.Session}
	}
	return out
}
