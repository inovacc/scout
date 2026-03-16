package detect

import (
	"encoding/json"
	"testing"
)

func TestFrameworkInfo_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input FrameworkInfo
		want  string
	}{
		{
			name:  "full",
			input: FrameworkInfo{Name: "React", Version: "18.2.0", SPA: true},
			want:  `{"name":"React","version":"18.2.0","spa":true}`,
		},
		{
			name:  "no_version",
			input: FrameworkInfo{Name: "Vue", SPA: false},
			want:  `{"name":"Vue","spa":false}`,
		},
		{
			name:  "empty",
			input: FrameworkInfo{},
			want:  `{"name":"","spa":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			if string(data) != tt.want {
				t.Errorf("json = %s, want %s", data, tt.want)
			}

			var got FrameworkInfo
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got != tt.input {
				t.Errorf("round-trip mismatch: got %+v, want %+v", got, tt.input)
			}
		})
	}
}

func TestTechStack_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input TechStack
	}{
		{
			name: "full_stack",
			input: TechStack{
				Frameworks:   []FrameworkInfo{{Name: "Next.js", Version: "14.0", SPA: true}},
				CSSFramework: "Tailwind CSS",
				BuildTool:    "Webpack",
				CMS:          "WordPress",
				Analytics:    []string{"Google Analytics", "Hotjar"},
				CDN:          "Cloudflare",
			},
		},
		{
			name:  "empty_stack",
			input: TechStack{},
		},
		{
			name: "multiple_frameworks",
			input: TechStack{
				Frameworks: []FrameworkInfo{
					{Name: "React", Version: "18.2.0", SPA: true},
					{Name: "Redux", Version: "5.0.0", SPA: false},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var got TechStack
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			// Re-marshal both for comparison (slice comparison)
			gotData, _ := json.Marshal(got)
			if string(gotData) != string(data) {
				t.Errorf("round-trip mismatch:\n  got:  %s\n  want: %s", gotData, data)
			}
		})
	}
}

func TestTechStack_OmitEmpty(t *testing.T) {
	ts := TechStack{}
	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Empty TechStack should produce minimal JSON (all fields have omitempty)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	// All fields have omitempty, so empty struct should have no keys
	if len(m) != 0 {
		t.Errorf("expected empty JSON object, got %d keys: %v", len(m), m)
	}
}

func TestRenderMode_Constants(t *testing.T) {
	tests := []struct {
		mode RenderMode
		want string
	}{
		{RenderCSR, "CSR"},
		{RenderSSR, "SSR"},
		{RenderSSG, "SSG"},
		{RenderISR, "ISR"},
		{RenderUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.mode) != tt.want {
				t.Errorf("RenderMode = %q, want %q", tt.mode, tt.want)
			}
		})
	}
}

func TestRenderInfo_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input RenderInfo
	}{
		{
			name:  "CSR_hydrated",
			input: RenderInfo{Mode: RenderCSR, Hydrated: true, Details: "React hydration detected"},
		},
		{
			name:  "SSR_not_hydrated",
			input: RenderInfo{Mode: RenderSSR, Hydrated: false},
		},
		{
			name:  "unknown",
			input: RenderInfo{Mode: RenderUnknown},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var got RenderInfo
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got != tt.input {
				t.Errorf("round-trip mismatch: got %+v, want %+v", got, tt.input)
			}
		})
	}
}

func TestRenderInfo_OmitEmptyDetails(t *testing.T) {
	ri := RenderInfo{Mode: RenderSSR, Hydrated: false}
	data, err := json.Marshal(ri)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := m["details"]; ok {
		t.Error("details should be omitted when empty")
	}
}

func TestPWAInfo_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input PWAInfo
	}{
		{
			name: "full_pwa",
			input: PWAInfo{
				HasServiceWorker: true,
				HasManifest:      true,
				Installable:      true,
				HTTPS:            true,
				PushCapable:      true,
				Manifest: &WebAppManifest{
					Name:            "My App",
					ShortName:       "App",
					Display:         "standalone",
					StartURL:        "/",
					ThemeColor:      "#ffffff",
					BackgroundColor: "#000000",
					Icons:           3,
				},
			},
		},
		{
			name:  "no_pwa",
			input: PWAInfo{},
		},
		{
			name: "partial_pwa",
			input: PWAInfo{
				HasServiceWorker: true,
				HTTPS:            true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var got PWAInfo
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			// Compare via re-marshal
			gotData, _ := json.Marshal(got)
			if string(gotData) != string(data) {
				t.Errorf("round-trip mismatch:\n  got:  %s\n  want: %s", gotData, data)
			}
		})
	}
}

func TestWebAppManifest_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input WebAppManifest
	}{
		{
			name: "full_manifest",
			input: WebAppManifest{
				Name:            "Test App",
				ShortName:       "Test",
				Display:         "standalone",
				StartURL:        "/app",
				ThemeColor:      "#3498db",
				BackgroundColor: "#ffffff",
				Icons:           5,
			},
		},
		{
			name:  "minimal_manifest",
			input: WebAppManifest{Name: "Minimal"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var got WebAppManifest
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got != tt.input {
				t.Errorf("round-trip mismatch: got %+v, want %+v", got, tt.input)
			}
		})
	}
}

func TestWebAppManifest_OmitEmpty(t *testing.T) {
	m := WebAppManifest{Name: "App"}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// These should be omitted
	for _, field := range []string{"short_name", "display", "start_url", "theme_color", "background_color"} {
		if _, ok := raw[field]; ok {
			t.Errorf("field %q should be omitted when empty", field)
		}
	}

	// These should always be present
	for _, field := range []string{"name", "icons"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("field %q should always be present", field)
		}
	}
}

func TestFrameworkInfo_UnmarshalFromExternalJSON(t *testing.T) {
	// Simulate JSON that might come from a browser detection script
	tests := []struct {
		name    string
		json    string
		want    FrameworkInfo
		wantErr bool
	}{
		{
			name: "standard",
			json: `{"name":"Angular","version":"17.0.0","spa":true}`,
			want: FrameworkInfo{Name: "Angular", Version: "17.0.0", SPA: true},
		},
		{
			name: "missing_optional_fields",
			json: `{"name":"jQuery","spa":false}`,
			want: FrameworkInfo{Name: "jQuery", SPA: false},
		},
		{
			name: "extra_fields_ignored",
			json: `{"name":"Svelte","version":"4.0","spa":true,"extra":"ignored"}`,
			want: FrameworkInfo{Name: "Svelte", Version: "4.0", SPA: true},
		},
		{
			name:    "invalid_json",
			json:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got FrameworkInfo
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRenderMode_JSONMarshal(t *testing.T) {
	ri := RenderInfo{Mode: RenderCSR, Hydrated: true}
	data, err := json.Marshal(ri)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["mode"] != "CSR" {
		t.Errorf("mode = %v, want %q", m["mode"], "CSR")
	}

	if m["hydrated"] != true {
		t.Errorf("hydrated = %v, want true", m["hydrated"])
	}
}
