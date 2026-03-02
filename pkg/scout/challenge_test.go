package scout

import (
	"net/http"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/challenge-cloudflare", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Just a moment...</title></head><body>
<div id="cf-browser-verification">Checking your browser</div>
<div id="challenge-running"></div>
<script src="/cdn-cgi/challenge-platform/scripts/managed/challenge.js"></script>
</body></html>`))
		})

		mux.HandleFunc("/challenge-turnstile", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<div class="cf-turnstile" data-sitekey="0x000000000"></div>
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js"></script>
</body></html>`))
		})

		mux.HandleFunc("/challenge-recaptcha-v2", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<div class="g-recaptcha" data-sitekey="6Le-wvkS"></div>
<script src="https://www.google.com/recaptcha/api.js" async defer></script>
</body></html>`))
		})

		mux.HandleFunc("/challenge-hcaptcha", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<div class="h-captcha" data-sitekey="test"></div>
<script src="https://hcaptcha.com/1/api.js" async defer></script>
</body></html>`))
		})

		mux.HandleFunc("/challenge-datadome", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<script src="https://js.datadome.co/tags.js"></script>
</body></html>`))
		})

		mux.HandleFunc("/challenge-none", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><h1>Normal Page</h1><p>No challenges here.</p></body></html>`))
		})

		mux.HandleFunc("/challenge-multi", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Just a moment...</title></head><body>
<div id="cf-browser-verification">Checking</div>
<div class="cf-turnstile" data-sitekey="0x000"></div>
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js"></script>
</body></html>`))
		})
	})
}

func TestDetectChallenges(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	tests := []struct {
		name     string
		path     string
		wantType ChallengeType
		wantMin  int
	}{
		{"cloudflare", "/challenge-cloudflare", ChallengeCloudflare, 1},
		{"turnstile", "/challenge-turnstile", ChallengeTurnstile, 1},
		{"recaptcha_v2", "/challenge-recaptcha-v2", ChallengeRecaptchaV2, 1},
		{"hcaptcha", "/challenge-hcaptcha", ChallengeHCaptcha, 1},
		{"datadome", "/challenge-datadome", ChallengeDataDome, 1},
		{"none", "/challenge-none", ChallengeNone, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := b.NewPage(srv.URL + tt.path)
			if err != nil {
				t.Fatal(err)
			}

			if err := page.WaitLoad(); err != nil {
				t.Fatal(err)
			}

			challenges, err := page.DetectChallenges()
			if err != nil {
				t.Fatal(err)
			}

			if len(challenges) < tt.wantMin {
				t.Errorf("expected at least %d challenges, got %d", tt.wantMin, len(challenges))
			}

			if tt.wantType != ChallengeNone {
				found := false

				for _, c := range challenges {
					if c.Type == tt.wantType {
						found = true

						if c.Confidence <= 0 || c.Confidence > 1.0 {
							t.Errorf("confidence should be (0,1], got %f", c.Confidence)
						}
					}
				}

				if !found {
					t.Errorf("expected challenge type %s not found in %v", tt.wantType, challenges)
				}
			}
		})
	}
}

func TestDetectChallenge_Highest(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/challenge-cloudflare")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	c, err := page.DetectChallenge()
	if err != nil {
		t.Fatal(err)
	}

	if c == nil {
		t.Fatal("expected a challenge, got nil")
	}

	if c.Type != ChallengeCloudflare {
		t.Errorf("expected cloudflare, got %s", c.Type)
	}
}

func TestHasChallenge(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	t.Run("has challenge", func(t *testing.T) {
		page, err := b.NewPage(srv.URL + "/challenge-cloudflare")
		if err != nil {
			t.Fatal(err)
		}

		if err := page.WaitLoad(); err != nil {
			t.Fatal(err)
		}

		has, err := page.HasChallenge()
		if err != nil {
			t.Fatal(err)
		}

		if !has {
			t.Error("expected true")
		}
	})

	t.Run("no challenge", func(t *testing.T) {
		page, err := b.NewPage(srv.URL + "/challenge-none")
		if err != nil {
			t.Fatal(err)
		}

		if err := page.WaitLoad(); err != nil {
			t.Fatal(err)
		}

		has, err := page.HasChallenge()
		if err != nil {
			t.Fatal(err)
		}

		if has {
			t.Error("expected false")
		}
	})
}

func TestDetectChallenges_Multiple(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/challenge-multi")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	challenges, err := page.DetectChallenges()
	if err != nil {
		t.Fatal(err)
	}

	if len(challenges) < 2 {
		t.Errorf("expected at least 2 challenges, got %d", len(challenges))
	}

	types := make(map[ChallengeType]bool)
	for _, c := range challenges {
		types[c.Type] = true
	}

	if !types[ChallengeCloudflare] {
		t.Error("expected cloudflare challenge")
	}

	if !types[ChallengeTurnstile] {
		t.Error("expected turnstile challenge")
	}
}

func TestDetectChallenges_NilPage(t *testing.T) {
	var p *Page

	_, err := p.DetectChallenges()
	if err == nil {
		t.Error("expected error for nil page")
	}
}
