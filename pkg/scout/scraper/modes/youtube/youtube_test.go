package youtube

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestYouTubeMode_Name(t *testing.T) {
	m := &YouTubeMode{}
	if got := m.Name(); got != "youtube" {
		t.Errorf("Name() = %q, want %q", got, "youtube")
	}
}

func TestYouTubeMode_Description(t *testing.T) {
	m := &YouTubeMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestYouTubeMode_AuthProvider(t *testing.T) {
	m := &YouTubeMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "youtube" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "youtube")
	}
}

// --- youtubeProvider tests ---

func TestYouTubeProvider_LoginURL(t *testing.T) {
	p := &youtubeProvider{}
	if got := p.LoginURL(); got != "https://accounts.google.com/" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &youtubeProvider{}
	err := p.ValidateSession(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_ValidSID(t *testing.T) {
	p := &youtubeProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "SID", Value: "abc"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ValidSSID(t *testing.T) {
	p := &youtubeProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "SSID", Value: "xyz"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ValidHSID(t *testing.T) {
	p := &youtubeProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "HSID", Value: "token"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ValidLOGININFO(t *testing.T) {
	p := &youtubeProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "LOGIN_INFO", Value: "info"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_NoCookies(t *testing.T) {
	p := &youtubeProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "other", Value: "val"}},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for missing youtube cookies")
	}
	if _, ok := err.(*scraper.AuthError); !ok {
		t.Errorf("expected *scraper.AuthError, got %T", err)
	}
}

func TestValidateSession_EmptyCookies(t *testing.T) {
	p := &youtubeProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{}}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for empty cookies")
	}
}

// --- buildTargetSet tests ---

func TestBuildTargetSet_Empty(t *testing.T) {
	set := buildTargetSet(nil)
	if set != nil {
		t.Errorf("expected nil, got %v", set)
	}
}

func TestBuildTargetSet_WithTargets(t *testing.T) {
	set := buildTargetSet([]string{"  UCxyz  ", "channelABC"})
	if _, ok := set["ucxyz"]; !ok {
		t.Error("expected 'ucxyz' in set")
	}
	if _, ok := set["channelabc"]; !ok {
		t.Error("expected 'channelabc' in set")
	}
}

// --- videoRendererToResult tests ---

func TestVideoRendererToResult(t *testing.T) {
	v := &videoRenderer{VideoID: "dQw4w9WgXcQ"}
	v.Title.Runs = []struct {
		Text string `json:"text"`
	}{{Text: "Never Gonna Give You Up"}}
	v.ShortBylineText.SimpleText = "Rick Astley"
	v.ViewCountText.SimpleText = "1.5B views"
	v.PublishedTimeText.SimpleText = "14 years ago"

	r := videoRendererToResult(v)
	if r.Type != scraper.ResultPost {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultPost)
	}
	if r.Source != "youtube" {
		t.Errorf("Source = %q", r.Source)
	}
	if r.ID != "dQw4w9WgXcQ" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Author != "Rick Astley" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Content != "Never Gonna Give You Up" {
		t.Errorf("Content = %q", r.Content)
	}
	if r.URL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Errorf("URL = %q", r.URL)
	}
	if r.Metadata["view_count"] != "1.5B views" {
		t.Errorf("Metadata[view_count] = %v", r.Metadata["view_count"])
	}
}

func TestVideoRendererToResult_EmptyTitle(t *testing.T) {
	v := &videoRenderer{VideoID: "abc123"}
	r := videoRendererToResult(v)
	if r.Content != "" {
		t.Errorf("Content = %q, want empty", r.Content)
	}
}

// --- channelRendererToResult tests ---

func TestChannelRendererToResult(t *testing.T) {
	c := &channelRenderer{ChannelID: "UC123"}
	c.Title.SimpleText = "Test Channel"
	c.SubscriberCountText.SimpleText = "1M subscribers"

	r := channelRendererToResult(c)
	if r.Type != scraper.ResultProfile {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultProfile)
	}
	if r.ID != "UC123" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.URL != "https://www.youtube.com/channel/UC123" {
		t.Errorf("URL = %q", r.URL)
	}
}

// --- playlistRendererToResult tests ---

func TestPlaylistRendererToResult(t *testing.T) {
	p := &playlistRenderer{PlaylistID: "PLabc"}
	p.Title.SimpleText = "My Playlist"
	p.Subtitle.SimpleText = "10 videos"

	r := playlistRendererToResult(p)
	if r.Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultChannel)
	}
	if r.ID != "PLabc" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.URL != "https://www.youtube.com/playlist?list=PLabc" {
		t.Errorf("URL = %q", r.URL)
	}
}

// --- parseSearchResults tests ---

func TestParseSearchResults_Valid(t *testing.T) {
	body := `{
		"contents": {
			"twoColumnSearchResultsRenderer": {
				"primaryContents": {
					"sectionListRenderer": {
						"contents": [
							{
								"itemSectionRenderer": {
									"contents": [
										{"videoRenderer": {"videoId": "v1", "title": {"runs": [{"text": "Video1"}]}, "shortBylineText": {"simpleText": "Author1"}, "viewCountText": {"simpleText": "100"}, "publishedTimeText": {"simpleText": "1d"}}},
										{"channelRenderer": {"channelId": "ch1", "title": {"simpleText": "Channel1"}, "subscriberCountText": {"simpleText": "1K"}}}
									]
								}
							}
						]
					}
				}
			}
		}
	}`

	results := parseSearchResults(body, nil)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Type != scraper.ResultPost {
		t.Errorf("results[0].Type = %q", results[0].Type)
	}
	if results[1].Type != scraper.ResultProfile {
		t.Errorf("results[1].Type = %q", results[1].Type)
	}
}

func TestParseSearchResults_InvalidJSON(t *testing.T) {
	results := parseSearchResults("bad json", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseSearchResults_EmptyContents(t *testing.T) {
	results := parseSearchResults(`{}`, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- parseBrowseResults tests ---

func TestParseBrowseResults_Valid(t *testing.T) {
	body := `{
		"contents": {
			"twoColumnBrowseResultsRenderer": {
				"tabs": [
					{
						"tabRenderer": {
							"content": {
								"sectionListRenderer": {
									"contents": [
										{
											"itemSectionRenderer": {
												"contents": [
													{"videoRenderer": {"videoId": "v2", "title": {"runs": [{"text": "BrowseVideo"}]}}}
												]
											}
										}
									]
								}
							}
						}
					}
				]
			}
		}
	}`

	results := parseBrowseResults(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ID != "v2" {
		t.Errorf("ID = %q", results[0].ID)
	}
}

func TestParseBrowseResults_InvalidJSON(t *testing.T) {
	results := parseBrowseResults("not json", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- parsePlayerResults tests ---

func TestParsePlayerResults_ReturnsNil(t *testing.T) {
	results := parsePlayerResults("{}", nil)
	if results != nil {
		t.Errorf("expected nil results from parsePlayerResults, got %v", results)
	}
}

// --- parseHijackEvent tests ---

func TestParseHijackEvent_NonResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventRequest}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseHijackEvent_EmptyBody(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://youtube.com/youtubei/v1/search", Body: ""},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseHijackEvent_UnknownURL(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://youtube.com/something/else", Body: "{}"},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for unknown URL, got %d", len(results))
	}
}
