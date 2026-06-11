package twitter

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
)

// --- parseGraphQLResponse tests ---

func TestParseGraphQLResponse(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		targetSet   map[string]struct{}
		wantResults int
		wantTweet   bool
		wantProfile bool
	}{
		{
			name:        "invalid JSON returns nil",
			body:        "not json",
			wantResults: 0,
		},
		{
			name:        "data is not a map (null) returns no results",
			body:        `{"data": null}`,
			wantResults: 0,
		},
		{
			name:        "data is an array (not a map) returns no results",
			body:        `{"data": [1, 2, 3]}`,
			wantResults: 0,
		},
		{
			name:        "empty data object returns no results",
			body:        `{"data": {}}`,
			wantResults: 0,
		},
		{
			name: "nested tweet is extracted",
			body: `{
				"data": {
					"threaded_conversation": {
						"instructions": [
							{
								"entry": {
									"content": {
										"tweet_results": {
											"result": {
												"id_str": "555",
												"full_text": "graphql tweet body",
												"created_at": "Mon Jan 15 10:30:00 +0000 2024",
												"retweet_count": 3,
												"favorite_count": 7,
												"reply_count": 1,
												"user": {"screen_name": "alice"}
											}
										}
									}
								}
							}
						]
					}
				}
			}`,
			wantResults: 1,
			wantTweet:   true,
		},
		{
			name: "nested profile is extracted",
			body: `{
				"data": {
					"user": {
						"result": {
							"legacy": {
								"screen_name": "bob",
								"followers_count": 4321,
								"description": "bio text",
								"statuses_count": 99,
								"verified": true,
								"created_at": "Mon Jan 15 10:30:00 +0000 2024"
							}
						}
					}
				}
			}`,
			wantResults: 1,
			wantProfile: true,
		},
		{
			name: "both tweet and profile extracted from one response",
			body: `{
				"data": {
					"tweet": {
						"id_str": "777",
						"full_text": "combined body",
						"user": {"screen_name": "carol"}
					},
					"profile": {
						"screen_name": "carol",
						"followers_count": 12,
						"description": "carol bio"
					}
				}
			}`,
			wantResults: 2,
			wantTweet:   true,
			wantProfile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := parseGraphQLResponse(tt.body, tt.targetSet)
			if len(results) != tt.wantResults {
				t.Fatalf("parseGraphQLResponse() returned %d results, want %d: %+v", len(results), tt.wantResults, results)
			}

			var gotTweet, gotProfile bool
			for _, r := range results {
				if r.Type == scraper.ResultPost {
					gotTweet = true
				}
				if r.Type == scraper.ResultProfile {
					gotProfile = true
				}
			}

			if gotTweet != tt.wantTweet {
				t.Errorf("gotTweet = %v, want %v", gotTweet, tt.wantTweet)
			}
			if gotProfile != tt.wantProfile {
				t.Errorf("gotProfile = %v, want %v", gotProfile, tt.wantProfile)
			}
		})
	}
}

// --- extractGraphQLTweets tests ---

func TestExtractGraphQLTweets(t *testing.T) {
	tweetData := map[string]any{
		"id_str":         "100",
		"full_text":      "hello graphql",
		"created_at":     "Mon Jan 15 10:30:00 +0000 2024",
		"retweet_count":  float64(2),
		"favorite_count": float64(4),
		"reply_count":    float64(1),
		"user": map[string]any{
			"screen_name": "Alice",
		},
	}

	tests := []struct {
		name        string
		data        map[string]any
		targetSet   map[string]struct{}
		wantResults int
		wantID      string
		wantAuthor  string
	}{
		{
			name:        "no tweet-like structures returns no results",
			data:        map[string]any{"foo": "bar", "nested": map[string]any{"baz": 1}},
			wantResults: 0,
		},
		{
			name:        "id_str without full_text is ignored",
			data:        map[string]any{"id_str": "1"},
			wantResults: 0,
		},
		{
			name: "directly nested tweet extracted with author",
			data: map[string]any{
				"timeline": map[string]any{
					"entries": []any{tweetData},
				},
			},
			wantResults: 1,
			wantID:      "100",
			wantAuthor:  "Alice",
		},
		{
			name: "target filter matches author (case-insensitive)",
			data: map[string]any{
				"entry": tweetData,
			},
			targetSet:   map[string]struct{}{"alice": {}},
			wantResults: 1,
			wantID:      "100",
			wantAuthor:  "Alice",
		},
		{
			name: "target filter excludes non-matching author",
			data: map[string]any{
				"entry": tweetData,
			},
			targetSet:   map[string]struct{}{"bob": {}},
			wantResults: 0,
		},
		{
			name: "tweet with no user/author still extracted when targetSet nil",
			data: map[string]any{
				"id_str":    "200",
				"full_text": "anon tweet",
			},
			wantResults: 1,
			wantID:      "200",
			wantAuthor:  "",
		},
		{
			name: "tweet with empty author bypasses target filter",
			data: map[string]any{
				"id_str":    "300",
				"full_text": "no author tweet",
			},
			targetSet:   map[string]struct{}{"someone": {}},
			wantResults: 1,
			wantID:      "300",
			wantAuthor:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := extractGraphQLTweets(tt.data, tt.targetSet)
			if len(results) != tt.wantResults {
				t.Fatalf("extractGraphQLTweets() returned %d, want %d: %+v", len(results), tt.wantResults, results)
			}
			if tt.wantResults == 0 {
				return
			}
			got := results[0]
			if got.Type != scraper.ResultPost {
				t.Errorf("Type = %q, want %q", got.Type, scraper.ResultPost)
			}
			if got.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tt.wantID)
			}
			if got.Author != tt.wantAuthor {
				t.Errorf("Author = %q, want %q", got.Author, tt.wantAuthor)
			}
			if got.Source != "twitter" {
				t.Errorf("Source = %q, want %q", got.Source, "twitter")
			}
		})
	}
}

func TestExtractGraphQLTweets_TimestampParsed(t *testing.T) {
	data := map[string]any{
		"id_str":     "1",
		"full_text":  "ts tweet",
		"created_at": "Mon Jan 15 10:30:00 +0000 2024",
	}
	results := extractGraphQLTweets(data, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp from created_at")
	}
	if results[0].Timestamp.Year() != 2024 {
		t.Errorf("Year = %d, want 2024", results[0].Timestamp.Year())
	}
}

// --- extractGraphQLProfiles tests ---

func TestExtractGraphQLProfiles(t *testing.T) {
	profileData := map[string]any{
		"screen_name":     "bob",
		"followers_count": float64(500),
		"description":     "bob bio",
		"statuses_count":  float64(80),
		"verified":        true,
		"created_at":      "Mon Jan 15 10:30:00 +0000 2024",
	}

	tests := []struct {
		name           string
		data           map[string]any
		wantResults    int
		wantID         string
		wantFollowers  int
		wantHasContent bool
	}{
		{
			name:        "no user-like structures returns no results",
			data:        map[string]any{"foo": "bar"},
			wantResults: 0,
		},
		{
			name:        "screen_name without followers_count is ignored",
			data:        map[string]any{"screen_name": "x"},
			wantResults: 0,
		},
		{
			name: "followers_count as non-float (int via JSON would be float, but raw int ignored)",
			data: map[string]any{
				"screen_name":     "y",
				"followers_count": 10,
			},
			wantResults: 0,
		},
		{
			name: "nested profile extracted",
			data: map[string]any{
				"user": map[string]any{
					"result": map[string]any{
						"legacy": profileData,
					},
				},
			},
			wantResults:    1,
			wantID:         "bob",
			wantFollowers:  500,
			wantHasContent: true,
		},
		{
			name: "profile within array extracted",
			data: map[string]any{
				"users": []any{profileData},
			},
			wantResults:    1,
			wantID:         "bob",
			wantFollowers:  500,
			wantHasContent: true,
		},
		{
			name: "profile without description has empty content",
			data: map[string]any{
				"screen_name":     "nodesc",
				"followers_count": float64(3),
			},
			wantResults:    1,
			wantID:         "nodesc",
			wantFollowers:  3,
			wantHasContent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := extractGraphQLProfiles(tt.data)
			if len(results) != tt.wantResults {
				t.Fatalf("extractGraphQLProfiles() returned %d, want %d: %+v", len(results), tt.wantResults, results)
			}
			if tt.wantResults == 0 {
				return
			}
			got := results[0]
			if got.Type != scraper.ResultProfile {
				t.Errorf("Type = %q, want %q", got.Type, scraper.ResultProfile)
			}
			if got.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tt.wantID)
			}
			if got.Author != tt.wantID {
				t.Errorf("Author = %q, want %q", got.Author, tt.wantID)
			}
			if got.Metadata["followers_count"] != tt.wantFollowers {
				t.Errorf("followers_count = %v, want %d", got.Metadata["followers_count"], tt.wantFollowers)
			}
			if tt.wantHasContent && got.Content == "" {
				t.Error("expected non-empty content")
			}
			if !tt.wantHasContent && got.Content != "" {
				t.Errorf("expected empty content, got %q", got.Content)
			}
		})
	}
}

// --- parseHijackEvent routing tests (graphql + user_by_screen_name) ---

func TestParseHijackEvent_GraphQLRouting(t *testing.T) {
	body := `{
		"data": {
			"tweet": {
				"id_str": "999",
				"full_text": "routed via graphql",
				"user": {"screen_name": "dave"}
			}
		}
	}`
	ev := scout.HijackEvent{
		Type: scout.HijackEventResponse,
		Response: &scout.CapturedResponse{
			URL:  "https://x.com/i/api/graphql/abc123/TweetDetail",
			Body: body,
		},
	}

	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("parseHijackEvent(graphql) returned %d, want 1", len(results))
	}
	if results[0].ID != "999" {
		t.Errorf("ID = %q, want %q", results[0].ID, "999")
	}
	if results[0].Type != scraper.ResultPost {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultPost)
	}
}

func TestParseHijackEvent_UserByScreenNameRouting(t *testing.T) {
	body := `{"screen_name": "erin", "followers_count": 321, "description": "erin bio"}`
	ev := scout.HijackEvent{
		Type: scout.HijackEventResponse,
		Response: &scout.CapturedResponse{
			URL:  "https://x.com/i/api/1.1/users/user_by_screen_name/erin.json",
			Body: body,
		},
	}

	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("parseHijackEvent(user_by_screen_name) returned %d, want 1", len(results))
	}
	if results[0].ID != "erin" {
		t.Errorf("ID = %q, want %q", results[0].ID, "erin")
	}
	if results[0].Type != scraper.ResultProfile {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultProfile)
	}
}

func TestParseHijackEvent_FollowersRouting(t *testing.T) {
	body := `{"users": [{"screen_name": "frank", "followers_count": 7, "description": "f"}]}`
	ev := scout.HijackEvent{
		Type: scout.HijackEventResponse,
		Response: &scout.CapturedResponse{
			URL:  "https://x.com/i/api/1.1/followers/list.json",
			Body: body,
		},
	}

	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("parseHijackEvent(followers) returned %d, want 1", len(results))
	}
	if results[0].ID != "frank" {
		t.Errorf("ID = %q, want %q", results[0].ID, "frank")
	}
}

func TestParseHijackEvent_TimelineRouting(t *testing.T) {
	body := `{"tweets": {"1": {"id_str": "1", "full_text": "tl"}}, "users": {}}`
	ev := scout.HijackEvent{
		Type: scout.HijackEventResponse,
		Response: &scout.CapturedResponse{
			URL:  "https://x.com/i/api/2/timeline/home/home_timeline.json",
			Body: body,
		},
	}

	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("parseHijackEvent(timeline) returned %d, want 1", len(results))
	}
}

// --- parseFollowersResponse invalid-JSON branch ---

func TestParseFollowersResponse_InvalidJSON(t *testing.T) {
	results := parseFollowersResponse("not json", nil)
	if results != nil {
		t.Errorf("parseFollowersResponse(invalid) = %v, want nil", results)
	}
}
