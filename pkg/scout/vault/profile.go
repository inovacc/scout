package vault

import (
	"sort"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// Cookie reuses the public scout cookie type so injection needs no mapping.
type Cookie = scout.Cookie

// OriginStore holds per-origin web storage absorbed from a UserProfile.
type OriginStore struct {
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
}

// SecretProfile is a decrypted, in-memory secret profile. All arbitrary secret
// values and auth headers are held in LockedBuffers. Call Close to zero them.
type SecretProfile struct {
	ID        string
	Name      string
	Secrets   map[string]*LockedBuffer
	Cookies   []Cookie
	Storage   map[string]OriginStore
	Headers   map[string]*LockedBuffer
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SecretProfileInput is the plaintext upsert payload accepted by Vault.Set.
// An empty ID creates a new profile; a known ID updates it.
type SecretProfileInput struct {
	ID      string
	Name    string
	Secrets map[string][]byte
	Cookies []Cookie
	Storage map[string]OriginStore
	Headers map[string][]byte
}

// ProfileMeta describes a profile without exposing any secret value.
type ProfileMeta struct {
	ID         string
	Name       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	SecretKeys []string
	HeaderKeys []string
	CookieN    int
}

// Meta returns non-secret metadata for the profile.
func (p *SecretProfile) Meta() ProfileMeta {
	m := ProfileMeta{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, CookieN: len(p.Cookies)}
	for k := range p.Secrets {
		m.SecretKeys = append(m.SecretKeys, k)
	}
	for k := range p.Headers {
		m.HeaderKeys = append(m.HeaderKeys, k)
	}
	sort.Strings(m.SecretKeys)
	sort.Strings(m.HeaderKeys)
	return m
}

// Close zeros every LockedBuffer held by the profile.
func (p *SecretProfile) Close() {
	for _, lb := range p.Secrets {
		lb.Zero()
	}
	for _, lb := range p.Headers {
		lb.Zero()
	}
}
