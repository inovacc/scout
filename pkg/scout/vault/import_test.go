package vault

import (
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

func TestFromUserProfileAbsorbsSecretFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.scoutprofile")
	up := &scout.UserProfile{
		Version: 1,
		Name:    "demo",
		Cookies: []scout.Cookie{{Name: "sid", Value: "v", Domain: "example.com"}},
		Headers: map[string]string{"Authorization": "Bearer t"},
		Storage: map[string]scout.ProfileOriginStorage{
			"https://example.com": {LocalStorage: map[string]string{"k": "v"}},
		},
	}
	if err := scout.SaveProfile(up, path); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	in, err := FromUserProfile(path)
	if err != nil {
		t.Fatalf("FromUserProfile: %v", err)
	}
	if len(in.Cookies) != 1 || in.Cookies[0].Name != "sid" {
		t.Fatalf("cookies = %+v", in.Cookies)
	}
	if string(in.Headers["Authorization"]) != "Bearer t" {
		t.Fatalf("headers = %v", in.Headers)
	}
	if in.Storage["https://example.com"].LocalStorage["k"] != "v" {
		t.Fatalf("storage = %+v", in.Storage)
	}
	if in.Name != "demo" {
		t.Fatalf("name = %q, want demo", in.Name)
	}
}
