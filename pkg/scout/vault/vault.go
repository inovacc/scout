package vault

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrProfileNotFound is returned when an ID does not match any profile.
var ErrProfileNotFound = errors.New("scout: vault: profile not found")

type config struct{ path string }

// Option configures a Vault.
type Option func(*config)

// WithPath overrides the default <scouthome>/profiles/vault.bin location.
func WithPath(p string) Option { return func(c *config) { c.path = p } }

func resolve(opts []Option) (config, error) {
	c := config{}
	for _, o := range opts {
		o(&c)
	}
	if c.path == "" {
		p, err := defaultVaultPath()
		if err != nil {
			return c, err
		}
		c.path = p
	}
	return c, nil
}

// Vault is an opened secret store. It holds the passphrase in a LockedBuffer for
// the lifetime of the handle so it can re-encrypt on every mutation and on Rotate.
type Vault struct {
	mu   sync.Mutex
	path string
	pass *LockedBuffer
	data *vaultData
}

// Create initializes a new empty vault at the resolved path. It errors if a
// vault already exists there.
func Create(passphrase []byte, opts ...Option) (*Vault, error) {
	c, err := resolve(opts)
	if err != nil {
		return nil, err
	}
	if _, err := loadVault(c.path, passphrase); err == nil {
		return nil, fmt.Errorf("scout: vault: already initialized at %s", c.path)
	} else if !errors.Is(err, ErrVaultNotFound) {
		// A decrypt error means a file exists but the passphrase is wrong — still "already exists".
		return nil, fmt.Errorf("scout: vault: already initialized at %s", c.path)
	}
	data := &vaultData{Version: 1}
	if err := saveVault(c.path, data, passphrase); err != nil {
		return nil, err
	}
	return &Vault{path: c.path, pass: NewLockedBuffer(append([]byte(nil), passphrase...)), data: data}, nil
}

// Open decrypts an existing vault.
func Open(passphrase []byte, opts ...Option) (*Vault, error) {
	c, err := resolve(opts)
	if err != nil {
		return nil, err
	}
	data, err := loadVault(c.path, passphrase)
	if err != nil {
		return nil, err
	}
	return &Vault{path: c.path, pass: NewLockedBuffer(append([]byte(nil), passphrase...)), data: data}, nil
}

// Set upserts a profile and persists the vault. Returns the profile ID.
func (v *Vault) Set(in SecretProfileInput) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now().UTC()
	sp := storedProfile{
		ID: in.ID, Name: in.Name, Secrets: in.Secrets, Cookies: in.Cookies,
		Storage: in.Storage, Headers: in.Headers, UpdatedAt: now,
	}
	if sp.ID == "" {
		sp.ID = newID()
		sp.CreatedAt = now
		v.data.Profiles = append(v.data.Profiles, sp)
	} else {
		idx := v.indexOf(sp.ID)
		if idx < 0 {
			return "", ErrProfileNotFound
		}
		sp.CreatedAt = v.data.Profiles[idx].CreatedAt
		v.data.Profiles[idx] = sp
	}
	if err := saveVault(v.path, v.data, v.pass.Bytes()); err != nil {
		return "", err
	}
	return sp.ID, nil
}

// Get returns a decrypted profile copy. Caller must Close it.
func (v *Vault) Get(id string) (*SecretProfile, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	idx := v.indexOf(id)
	if idx < 0 {
		return nil, ErrProfileNotFound
	}
	return materialize(v.data.Profiles[idx]), nil
}

// List returns metadata for every profile (never any secret value).
func (v *Vault) List() []ProfileMeta {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]ProfileMeta, 0, len(v.data.Profiles))
	for _, sp := range v.data.Profiles {
		p := materialize(sp)
		out = append(out, p.Meta())
		p.Close()
	}
	return out
}

// Remove deletes a profile and persists the vault.
func (v *Vault) Remove(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	idx := v.indexOf(id)
	if idx < 0 {
		return ErrProfileNotFound
	}
	// Zero the stored secret bytes before dropping the entry.
	zeroStored(&v.data.Profiles[idx])
	v.data.Profiles = append(v.data.Profiles[:idx], v.data.Profiles[idx+1:]...)
	return saveVault(v.path, v.data, v.pass.Bytes())
}

// Close zeros the cached passphrase and decrypted secret bytes.
func (v *Vault) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i := range v.data.Profiles {
		zeroStored(&v.data.Profiles[i])
	}
	v.pass.Zero()
	return nil
}

func (v *Vault) indexOf(id string) int {
	for i := range v.data.Profiles {
		if v.data.Profiles[i].ID == id {
			return i
		}
	}
	return -1
}

func materialize(sp storedProfile) *SecretProfile {
	p := &SecretProfile{
		ID: sp.ID, Name: sp.Name, Cookies: sp.Cookies, Storage: sp.Storage,
		CreatedAt: sp.CreatedAt, UpdatedAt: sp.UpdatedAt,
		Secrets: map[string]*LockedBuffer{}, Headers: map[string]*LockedBuffer{},
	}
	for k, val := range sp.Secrets {
		p.Secrets[k] = NewLockedBuffer(append([]byte(nil), val...))
	}
	for k, val := range sp.Headers {
		p.Headers[k] = NewLockedBuffer(append([]byte(nil), val...))
	}
	return p
}

func zeroStored(sp *storedProfile) {
	for _, val := range sp.Secrets {
		zero(val)
	}
	for _, val := range sp.Headers {
		zero(val)
	}
}
