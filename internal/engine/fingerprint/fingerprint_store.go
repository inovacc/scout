package fingerprint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StoredFingerprint wraps a Fingerprint with persistence metadata.
type StoredFingerprint struct {
	ID          string       `json:"id"`
	Fingerprint *Fingerprint `json:"fingerprint"`
	CreatedAt   time.Time    `json:"created_at"`
	LastUsed    time.Time    `json:"last_used"`
	UseCount    int          `json:"use_count"`
	Domains     []string     `json:"domains,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
}

// FingerprintStore manages fingerprint persistence on disk.
// Default directory: ~/.scout/fingerprints/
type FingerprintStore struct {
	dir string
}

// NewFingerprintStore creates a store backed by the given directory.
// If dir is empty, defaults to ~/.scout/fingerprints/.
func NewFingerprintStore(dir string) (*FingerprintStore, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("scout: fingerprint store: home dir: %w", err)
		}

		dir = filepath.Join(home, ".scout", "fingerprints")
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("scout: fingerprint store: mkdir: %w", err)
	}

	return &FingerprintStore{dir: dir}, nil
}

// Save persists a fingerprint and returns its stored wrapper.
func (s *FingerprintStore) Save(fp *Fingerprint, tags ...string) (*StoredFingerprint, error) {
	now := time.Now()

	sf := &StoredFingerprint{
		ID:          uuid.NewString(),
		Fingerprint: fp,
		CreatedAt:   now,
		LastUsed:    now,
		Tags:        tags,
	}
	if err := s.write(sf); err != nil {
		return nil, err
	}

	return sf, nil
}

// Load reads a stored fingerprint by ID.
func (s *FingerprintStore) Load(id string) (*StoredFingerprint, error) {
	path := filepath.Join(s.dir, id+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scout: fingerprint store: load %s: %w", id, err)
	}

	var sf StoredFingerprint
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("scout: fingerprint store: decode %s: %w", id, err)
	}

	return &sf, nil
}

// List returns all stored fingerprints sorted by creation time (newest first).
func (s *FingerprintStore) List() ([]*StoredFingerprint, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("scout: fingerprint store: list: %w", err)
	}

	var results []*StoredFingerprint

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(e.Name(), ".json")

		sf, err := s.Load(id)
		if err != nil {
			continue
		}

		results = append(results, sf)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}

// Delete removes a stored fingerprint by ID.
func (s *FingerprintStore) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("scout: fingerprint store: delete %s: %w", id, err)
	}

	return nil
}

// MarkUsed updates the last-used timestamp, use count, and domain list.
func (s *FingerprintStore) MarkUsed(id string, domain string) error {
	sf, err := s.Load(id)
	if err != nil {
		return err
	}

	sf.LastUsed = time.Now()

	sf.UseCount++
	if domain != "" && !containsString(sf.Domains, domain) {
		sf.Domains = append(sf.Domains, domain)
	}

	return s.write(sf)
}

// Generate creates a new fingerprint, saves it, and returns the stored wrapper.
func (s *FingerprintStore) Generate(opts ...FingerprintOption) (*StoredFingerprint, error) {
	fp := GenerateFingerprint(opts...)
	return s.Save(fp)
}

func (s *FingerprintStore) write(sf *StoredFingerprint) error {
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: fingerprint store: marshal: %w", err)
	}

	path := filepath.Join(s.dir, sf.ID+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("scout: fingerprint store: write: %w", err)
	}

	return nil
}

func containsString(ss []string, s string) bool {
	return slices.Contains(ss, s)
}
