package flow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/vault"
)

func TestVaultResolverInjectsSecretHeader(t *testing.T) {
	vp := filepath.Join(t.TempDir(), "vault.bin")
	v, err := vault.Create([]byte("pw"), vault.WithPath(vp))
	if err != nil {
		t.Fatal(err)
	}
	id, err := v.Set(vault.SecretProfileInput{Name: "svc", Secrets: map[string][]byte{"API_KEY": []byte("sk-secret")}})
	if err != nil {
		t.Fatal(err)
	}
	_ = v.Close()

	v2, err := vault.Open([]byte("pw"), vault.WithPath(vp))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = v2.Close() }()
	h, err := v2.Use(id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()

	var sawKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawKey = r.Header.Get("X-Api-Key")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := &FlowSpec{Version: "1", Steps: []FlowStep{
		{ID: "call", Request: Request{Method: "GET", URL: srv.URL,
			Headers: map[string]string{"X-Api-Key": "${secret.API_KEY}"}}}}}
	if _, err := Run(context.Background(), f, RunOptions{Secrets: NewVaultResolver(h)}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sawKey != "sk-secret" {
		t.Fatalf("secret not injected: %q", sawKey)
	}
}
