package gmaps

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
)

// The gmaps parse helpers search for *literal* pattern strings (not regexes), e.g. the
// exact bytes `"name":"([^"]+)"`. These literal sequences contain unescaped quotes and so
// can never appear inside a valid JSON string value. The fixtures below therefore embed the
// literal patterns directly in the body text and assert on the exact (admittedly quirky)
// substrings the offset arithmetic produces. This pins current behaviour so future refactors
// surface intentionally.

// Literal pattern fragments the parsers look for, matching gmaps.go exactly.
const (
	litNamePattern    = `"name":"([^"]+)"`
	litAddressPattern = `"formatted_address":"([^"]+)"`
	litPhonePattern   = `"international_phone_number":"([^"]+)"`
	litReviewPattern  = `"review":"([^"]+)"`
	litPhotoPattern   = `"url":"(https://[^"]+)"`
)

// --- parseBusinessProfile: extraction branches ---

func TestParseBusinessProfile_ExtractsBranches(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantNil     bool
		wantContent string  // expected profile.Name surfaced as Result.Content
		wantAddress string  // expected metadata["address"]
		wantPhone   string  // expected metadata["phone"]
		wantRating  float64 // expected metadata["rating"]
	}{
		{
			name:        "name pattern yields non-nil profile",
			body:        `{` + litNamePattern + `,"rating":4.7}`,
			wantContent: "^",
			wantRating:  4.7,
		},
		{
			name:        "address pattern yields non-nil profile",
			body:        `z` + litAddressPattern + `w`,
			wantAddress: "^",
		},
		{
			name:        "name plus phone plus rating",
			body:        `{` + litNamePattern + `,` + litPhonePattern + `,"rating":3.0}`,
			wantContent: "^",
			wantPhone:   `]+)`,
			wantRating:  3.0,
		},
		{
			name:    "valid json with rating but no name or address is nil",
			body:    `{"rating":3.2,"foo":"bar"}`,
			wantNil: true,
		},
		{
			name:    "no recognizable patterns is nil",
			body:    `{"unrelated":"value"}`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBusinessProfile(tt.body)

			if tt.wantNil {
				if got != nil {
					t.Fatalf("parseBusinessProfile(%q) = %+v, want nil", tt.body, got)
				}

				return
			}

			if got == nil {
				t.Fatalf("parseBusinessProfile(%q) = nil, want non-nil", tt.body)
			}

			if got.Type != scraper.ResultProfile {
				t.Errorf("Type = %q, want %q", got.Type, scraper.ResultProfile)
			}

			if got.Source != "gmaps" {
				t.Errorf("Source = %q, want %q", got.Source, "gmaps")
			}

			if got.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", got.Content, tt.wantContent)
			}

			if tt.wantAddress != "" {
				if addr, _ := got.Metadata["address"].(string); addr != tt.wantAddress {
					t.Errorf("metadata[address] = %q, want %q", addr, tt.wantAddress)
				}
			}

			if tt.wantPhone != "" {
				if phone, _ := got.Metadata["phone"].(string); phone != tt.wantPhone {
					t.Errorf("metadata[phone] = %q, want %q", phone, tt.wantPhone)
				}
			}

			if tt.wantRating != 0 {
				if rating, _ := got.Metadata["rating"].(float64); rating != tt.wantRating {
					t.Errorf("metadata[rating] = %v, want %v", rating, tt.wantRating)
				}
			}

			// Raw should carry the businessProfile value.
			if _, ok := got.Raw.(businessProfile); !ok {
				t.Errorf("Raw type = %T, want businessProfile", got.Raw)
			}
		})
	}
}

// --- parseReviews ---

func TestParseReviews(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string // expected Result.Content values, in order
	}{
		{
			name: "no pattern yields nothing",
			body: `{"foo":"bar"}`,
			want: nil,
		},
		{
			name: "single review",
			body: `q` + litReviewPattern + `0123456789Xhello world"Y`,
			want: []string{"Xhello world"},
		},
		{
			name: "two reviews",
			body: `a` + litReviewPattern + `0123456789Afirst"b` + litReviewPattern + `0123456789Bsecond"c`,
			want: []string{"Afirst", "Bsecond"},
		},
		{
			name: "pattern present but no closing quote is skipped",
			body: `a` + litReviewPattern + `0123456789Xunterminated`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseReviews(tt.body)

			if len(got) != len(tt.want) {
				t.Fatalf("parseReviews returned %d results, want %d: %+v", len(got), len(tt.want), got)
			}

			for i, want := range tt.want {
				if got[i].Type != scraper.ResultComment {
					t.Errorf("result[%d].Type = %q, want %q", i, got[i].Type, scraper.ResultComment)
				}

				if got[i].Source != "gmaps" {
					t.Errorf("result[%d].Source = %q, want %q", i, got[i].Source, "gmaps")
				}

				if got[i].Content != want {
					t.Errorf("result[%d].Content = %q, want %q", i, got[i].Content, want)
				}

				if typ, _ := got[i].Metadata["type"].(string); typ != "review" {
					t.Errorf("result[%d].metadata[type] = %q, want %q", i, typ, "review")
				}
			}
		})
	}
}

// --- parsePhotos ---

func TestParsePhotos(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string // expected Result.URL values, in order
	}{
		{
			name: "no pattern yields nothing",
			body: `{"foo":"bar"}`,
			want: nil,
		},
		{
			name: "google.com url is captured",
			body: `p` + litPhotoPattern + `ABCDEFGmaps.google.com/photo.jpg"X`,
			want: []string{"Gmaps.google.com/photo.jpg"},
		},
		{
			name: "url without google or maps is dropped",
			body: `p` + litPhotoPattern + `ABCDEFexample.org/pic.png"X`,
			want: nil,
		},
		{
			name: "pattern present but no closing quote is skipped",
			body: `p` + litPhotoPattern + `ABCDEFGmaps-no-terminator`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePhotos(tt.body)

			if len(got) != len(tt.want) {
				t.Fatalf("parsePhotos returned %d results, want %d: %+v", len(got), len(tt.want), got)
			}

			for i, want := range tt.want {
				if got[i].Type != scraper.ResultFile {
					t.Errorf("result[%d].Type = %q, want %q", i, got[i].Type, scraper.ResultFile)
				}

				if got[i].Source != "gmaps" {
					t.Errorf("result[%d].Source = %q, want %q", i, got[i].Source, "gmaps")
				}

				if got[i].URL != want {
					t.Errorf("result[%d].URL = %q, want %q", i, got[i].URL, want)
				}

				if typ, _ := got[i].Metadata["type"].(string); typ != "photo" {
					t.Errorf("result[%d].metadata[type] = %q, want %q", i, typ, "photo")
				}
			}
		})
	}
}

// --- parseMapsPLACES: aggregation over valid JSON ---

func TestParseMapsPLACES_ValidJSON(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantCount int
		wantType  scraper.ResultType
	}{
		{
			name:      "valid json without any patterns yields nothing",
			body:      `{"rating":3.2,"foo":"bar"}`,
			wantCount: 0,
		},
		{
			name:      "empty object yields nothing",
			body:      `{}`,
			wantCount: 0,
		},
		{
			name:      "array json yields nothing",
			body:      `[1,2,3]`,
			wantCount: 0,
		},
		{
			name:      "non-json body is rejected before sub-parsers",
			body:      `q` + litNamePattern + `,"rating":4.7}`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMapsPLACES(tt.body, nil)
			if len(got) != tt.wantCount {
				t.Fatalf("parseMapsPLACES returned %d results, want %d: %+v", len(got), tt.wantCount, got)
			}
		})
	}
}

// --- parseHijackEvent: response routing with body ---

func TestParseHijackEvent_PreviewURLWithValidJSON(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		body      string
		wantCount int
	}{
		{
			name:      "preview url routes to parser but valid json has no matches",
			url:       "https://www.google.com/maps/preview/place",
			body:      `{"rating":3.2}`,
			wantCount: 0,
		},
		{
			name:      "rpc url routes to parser",
			url:       "https://www.google.com/maps/rpc/listugcposts",
			body:      `{}`,
			wantCount: 0,
		},
		{
			name:      "unrelated url is ignored",
			url:       "https://www.google.com/search?q=x",
			body:      `{}`,
			wantCount: 0,
		},
		{
			name:      "nil response yields nothing",
			url:       "",
			body:      "",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := scout.HijackEvent{
				Type:     scout.HijackEventResponse,
				Response: &scout.CapturedResponse{URL: tt.url, Body: tt.body},
			}

			got := parseHijackEvent(ev, nil)
			if len(got) != tt.wantCount {
				t.Fatalf("parseHijackEvent returned %d results, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestParseHijackEvent_NilResponsePointer(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventResponse, Response: nil}
	if got := parseHijackEvent(ev, nil); len(got) != 0 {
		t.Fatalf("parseHijackEvent with nil Response = %d results, want 0", len(got))
	}
}
