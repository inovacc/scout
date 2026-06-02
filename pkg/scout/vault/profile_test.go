package vault

import "testing"

func TestSecretProfileCloseZeros(t *testing.T) {
	p := &SecretProfile{
		ID:   "abc",
		Name: "demo",
		Secrets: map[string]*LockedBuffer{
			"api_key": NewLockedBuffer([]byte("sk-123")),
		},
		Headers: map[string]*LockedBuffer{
			"Authorization": NewLockedBuffer([]byte("Bearer xyz")),
		},
	}
	backing := p.Secrets["api_key"].Bytes()
	p.Close()
	for i, b := range backing {
		if b != 0 {
			t.Fatalf("secret byte %d not zeroed after Close", i)
		}
	}
}

func TestProfileMetaHidesValues(t *testing.T) {
	p := &SecretProfile{
		ID:      "id1",
		Name:    "n",
		Secrets: map[string]*LockedBuffer{"k": NewLockedBuffer([]byte("v"))},
	}
	defer p.Close()
	m := p.Meta()
	if m.ID != "id1" || m.Name != "n" {
		t.Fatalf("meta id/name = %q/%q", m.ID, m.Name)
	}
	if len(m.SecretKeys) != 1 || m.SecretKeys[0] != "k" {
		t.Fatalf("SecretKeys = %v, want [k]", m.SecretKeys)
	}
}
