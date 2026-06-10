package capture

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestOriginToExtID(t *testing.T) {
	const id = "abcdefghijklmnopabcdefghijklmnop" // 32 chars, all in a-p
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"chrome-extension://" + id + "/", id, true},
		{"chrome-extension://" + id, id, true},
		{"chrome-extension://short/", "", false},
		{"https://example.com/", "", false},
		{"chrome-extension://ABCDEFGHIJKLMNOP/", "", false}, // uppercase not in a-p
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := OriginToExtID(c.in)
		if got != c.want || ok != c.ok {
			t.Fatalf("OriginToExtID(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestIsNativeMessagingLaunch(t *testing.T) {
	origin, ok := IsNativeMessagingLaunch([]string{"scout", "chrome-extension://abcdefghijklmnopabcdefghijklmnop/", "--parent-window=123"})
	if !ok || origin != "chrome-extension://abcdefghijklmnopabcdefghijklmnop/" {
		t.Fatalf("got (%q,%v)", origin, ok)
	}
	if _, ok := IsNativeMessagingLaunch([]string{"scout", "vault", "list"}); ok {
		t.Fatal("should not detect a normal subcommand invocation")
	}
	if _, ok := IsNativeMessagingLaunch([]string{"scout"}); ok {
		t.Fatal("bare invocation is not a launch")
	}
}

func TestExtensionIDAndManifestKey(t *testing.T) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	id := ExtensionID(der)
	if len(id) != 32 {
		t.Fatalf("id len = %d, want 32", len(id))
	}
	for _, c := range id {
		if c < 'a' || c > 'p' {
			t.Fatalf("id char %q out of a-p", c)
		}
	}
	if ExtensionID(der) != id {
		t.Fatal("ExtensionID not deterministic")
	}

	// ManifestKey is base64(DER) and must round-trip back to the DER bytes.
	mk := ManifestKey(der)
	back, err := base64.StdEncoding.DecodeString(mk)
	if err != nil {
		t.Fatalf("manifest key not valid base64: %v", err)
	}
	if string(back) != string(der) {
		t.Fatal("manifest key does not round-trip to DER")
	}

	// A different key yields a different ID.
	k2, _ := rsa.GenerateKey(rand.Reader, 2048)
	der2, _ := x509.MarshalPKIXPublicKey(&k2.PublicKey)
	if ExtensionID(der2) == id {
		t.Fatal("distinct keys produced the same id")
	}

	// The ID derived here must satisfy OriginToExtID.
	if _, ok := OriginToExtID("chrome-extension://" + id + "/"); !ok {
		t.Fatal("derived id rejected by OriginToExtID")
	}
}
