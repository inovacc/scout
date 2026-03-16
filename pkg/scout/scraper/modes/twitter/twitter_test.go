package twitter

import (
	"context"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestTwitterMode_Name(t *testing.T) {
	m := &TwitterMode{}
	if got := m.Name(); got != "twitter" {
		t.Errorf("Name() = %q, want %q", got, "twitter")
	}
}

func TestTwitterMode_Description(t *testing.T) {
	m := &TwitterMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestTwitterMode_AuthProvider(t *testing.T) {
	m := &TwitterMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "twitter" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "twitter")
	}
}

// --- twitterProvider tests ---

func TestTwitterProvider_Name(t *testing.T) {
	p := &twitterProvider{}
	if got := p.Name(); got != "twitter" {
		t.Errorf("Name() = %q, want %q", got, "twitter")
	}
}

func TestTwitterProvider_LoginURL(t *testing.T) {
	p := &twitterProvider{}
	if got := p.LoginURL(); got != "https://x.com/i/flow/login" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &twitterProvider{}
	err := p.ValidateSession(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_MissingAuthToken(t *testing.T) {
	p := &twitterProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"ct0": "val"},
	}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for missing auth_token")
	}
}

func TestValidateSession_MissingCT0(t *testing.T) {
	p := &twitterProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"auth_token": "val"},
	}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for missing ct0")
	}
}

func TestValidateSession_Valid(t *testing.T) {
	p := &twitterProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"auth_token": "abc", "ct0": "def"},
	}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_EmptyTokenValues(t *testing.T) {
	p := &twitterProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"auth_token": "", "ct0": "def"},
	}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for empty auth_token value")
	}
}

// --- buildTargetSet tests ---

func TestBuildTargetSet_Nil(t *testing.T) {
	set := buildTargetSet(nil)
	if set != nil {
		t.Errorf("buildTargetSet(nil) = %v, want nil", set)
	}
}

func TestBuildTargetSet_StripsAtSign(t *testing.T) {
	set := buildTargetSet([]string{"@JohnDoe"})
	if _, ok := set["johndoe"]; !ok {
		t.Error("expected lowercase stripped key in set")
	}
	if _, ok := set["@johndoe"]; ok {
		t.Error("@ prefix should be stripped")
	}
}

func TestBuildTargetSet_Lowercase(t *testing.T) {
	set := buildTargetSet([]string{"UserName"})
	if _, ok := set["username"]; !ok {
		t.Error("expected lowercase key")
	}
}

// --- parseTwitterTimestamp tests ---

func TestParseTwitterTimestamp_Nil(t *testing.T) {
	ts := parseTwitterTimestamp(nil)
	if !ts.IsZero() {
		t.Errorf("parseTwitterTimestamp(nil) = %v, want zero", ts)
	}
}

func TestParseTwitterTimestamp_EmptyString(t *testing.T) {
	ts := parseTwitterTimestamp("")
	if !ts.IsZero() {
		t.Errorf("parseTwitterTimestamp('') = %v, want zero", ts)
	}
}

func TestParseTwitterTimestamp_NotString(t *testing.T) {
	ts := parseTwitterTimestamp(12345)
	if !ts.IsZero() {
		t.Errorf("parseTwitterTimestamp(int) = %v, want zero", ts)
	}
}

func TestParseTwitterTimestamp_StandardFormat(t *testing.T) {
	ts := parseTwitterTimestamp("Mon Jan 15 10:30:00 +0000 2024")
	if ts.IsZero() {
		t.Error("parseTwitterTimestamp(standard) returned zero time")
	}
	if ts.Year() != 2024 || ts.Month() != time.January || ts.Day() != 15 {
		t.Errorf("parseTwitterTimestamp() = %v", ts)
	}
}

func TestParseTwitterTimestamp_RFC3339(t *testing.T) {
	ts := parseTwitterTimestamp("2024-01-15T10:30:00Z")
	if ts.IsZero() {
		t.Error("parseTwitterTimestamp(RFC3339) returned zero time")
	}
}

func TestParseTwitterTimestamp_UnixString(t *testing.T) {
	ts := parseTwitterTimestamp("1705312200")
	if ts.IsZero() {
		t.Error("parseTwitterTimestamp(unix string) returned zero time")
	}
}

func TestParseTwitterTimestamp_Invalid(t *testing.T) {
	ts := parseTwitterTimestamp("not-a-date")
	if !ts.IsZero() {
		t.Errorf("parseTwitterTimestamp(invalid) = %v, want zero", ts)
	}
}

// --- tweetMapToResult tests ---

func TestTweetMapToResult_Valid(t *testing.T) {
	tweet := map[string]any{
		"id_str":         "123",
		"full_text":      "Hello world",
		"user_id_str":    "u1",
		"created_at":     "Mon Jan 15 10:30:00 +0000 2024",
		"retweet_count":  float64(5),
		"favorite_count": float64(10),
		"reply_count":    float64(2),
	}

	result := tweetMapToResult(tweet, nil)
	if result == nil {
		t.Fatal("tweetMapToResult() returned nil")
	}
	if result.Type != scraper.ResultPost {
		t.Errorf("Type = %q, want %q", result.Type, scraper.ResultPost)
	}
	if result.ID != "123" {
		t.Errorf("ID = %q, want %q", result.ID, "123")
	}
	if result.Content != "Hello world" {
		t.Errorf("Content = %q, want %q", result.Content, "Hello world")
	}
	if result.Author != "u1" {
		t.Errorf("Author = %q, want %q", result.Author, "u1")
	}
}

func TestTweetMapToResult_MissingIDStr(t *testing.T) {
	tweet := map[string]any{"full_text": "Hello"}
	result := tweetMapToResult(tweet, nil)
	if result != nil {
		t.Errorf("expected nil for missing id_str, got %v", result)
	}
}

func TestTweetMapToResult_MissingFullText(t *testing.T) {
	tweet := map[string]any{"id_str": "123"}
	result := tweetMapToResult(tweet, nil)
	if result != nil {
		t.Errorf("expected nil for missing full_text, got %v", result)
	}
}

func TestTweetMapToResult_WithUserField(t *testing.T) {
	tweet := map[string]any{
		"id_str":    "123",
		"full_text": "Hello",
		"user":      "alice",
	}
	result := tweetMapToResult(tweet, nil)
	if result == nil {
		t.Fatal("tweetMapToResult() returned nil")
	}
	if result.Author != "alice" {
		t.Errorf("Author = %q, want %q", result.Author, "alice")
	}
}

func TestTweetMapToResult_TargetFilter(t *testing.T) {
	tweet := map[string]any{
		"id_str":    "123",
		"full_text": "Hello",
		"user":      "alice",
	}
	targetSet := map[string]struct{}{"bob": {}}
	result := tweetMapToResult(tweet, targetSet)
	if result != nil {
		t.Errorf("expected nil for filtered author, got %v", result)
	}
}

// --- userMapToResult tests ---

func TestUserMapToResult_Valid(t *testing.T) {
	user := map[string]any{
		"screen_name":     "alice",
		"followers_count": float64(100),
		"description":     "Hello",
		"statuses_count":  float64(50),
		"verified":        true,
	}

	result := userMapToResult(user)
	if result == nil {
		t.Fatal("userMapToResult() returned nil")
	}
	if result.Type != scraper.ResultProfile {
		t.Errorf("Type = %q, want %q", result.Type, scraper.ResultProfile)
	}
	if result.ID != "alice" {
		t.Errorf("ID = %q, want %q", result.ID, "alice")
	}
	if result.Metadata["followers_count"] != 100 {
		t.Errorf("followers_count = %v, want 100", result.Metadata["followers_count"])
	}
}

func TestUserMapToResult_MissingScreenName(t *testing.T) {
	user := map[string]any{"followers_count": float64(100)}
	result := userMapToResult(user)
	if result != nil {
		t.Errorf("expected nil for missing screen_name")
	}
}

// --- parseSearchResponse tests ---

func TestParseSearchResponse_Valid(t *testing.T) {
	body := `{
		"globalObjects": {
			"tweets": {
				"1": {"id_str": "1", "full_text": "Tweet 1", "user_id_str": "u1"},
				"2": {"id_str": "2", "full_text": "Tweet 2"}
			},
			"users": {
				"u1": {"screen_name": "alice", "followers_count": 50, "description": "Hi"}
			}
		}
	}`

	results := parseSearchResponse(body, nil)
	if len(results) == 0 {
		t.Fatal("parseSearchResponse() returned 0 results")
	}
	// Should have tweets + users
	hasTweet, hasProfile := false, false
	for _, r := range results {
		if r.Type == scraper.ResultPost {
			hasTweet = true
		}
		if r.Type == scraper.ResultProfile {
			hasProfile = true
		}
	}
	if !hasTweet {
		t.Error("expected at least one tweet result")
	}
	if !hasProfile {
		t.Error("expected at least one profile result")
	}
}

func TestParseSearchResponse_InvalidJSON(t *testing.T) {
	results := parseSearchResponse("not json", nil)
	if results != nil {
		t.Errorf("parseSearchResponse(invalid) = %v, want nil", results)
	}
}

// --- parseFollowersResponse tests ---

func TestParseFollowersResponse_Valid(t *testing.T) {
	body := `{"users": [{"screen_name": "bob", "followers_count": 200, "description": "Dev"}]}`
	results := parseFollowersResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("parseFollowersResponse() returned %d, want 1", len(results))
	}
	if results[0].ID != "bob" {
		t.Errorf("ID = %q, want %q", results[0].ID, "bob")
	}
}

func TestParseFollowersResponse_Empty(t *testing.T) {
	body := `{"users": []}`
	results := parseFollowersResponse(body, nil)
	if len(results) != 0 {
		t.Errorf("parseFollowersResponse(empty) returned %d, want 0", len(results))
	}
}

// --- parseTimelineResponse tests ---

func TestParseTimelineResponse_Valid(t *testing.T) {
	body := `{
		"tweets": {"1": {"id_str": "1", "full_text": "Hello timeline"}},
		"users": {"u1": {"screen_name": "alice", "followers_count": 50}}
	}`

	results := parseTimelineResponse(body, nil)
	if len(results) == 0 {
		t.Fatal("parseTimelineResponse() returned 0 results")
	}
}

func TestParseTimelineResponse_InvalidJSON(t *testing.T) {
	results := parseTimelineResponse("not json", nil)
	if results != nil {
		t.Errorf("parseTimelineResponse(invalid) = %v, want nil", results)
	}
}

// --- parseUserProfileResponse tests ---

func TestParseUserProfileResponse_Valid(t *testing.T) {
	body := `{"screen_name": "alice", "followers_count": 100, "description": "Hello"}`
	results := parseUserProfileResponse(body)
	if len(results) != 1 {
		t.Fatalf("parseUserProfileResponse() returned %d, want 1", len(results))
	}
}

func TestParseUserProfileResponse_InvalidJSON(t *testing.T) {
	results := parseUserProfileResponse("not json")
	if results != nil {
		t.Errorf("parseUserProfileResponse(invalid) = %v, want nil", results)
	}
}

func TestParseUserProfileResponse_NoScreenName(t *testing.T) {
	body := `{"name": "Alice"}`
	results := parseUserProfileResponse(body)
	if results != nil {
		t.Errorf("expected nil for user without screen_name")
	}
}

// --- parseHijackEvent routing tests ---

func TestParseHijackEvent_NonResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventRequest}
	results := parseHijackEvent(ev, nil)
	if results != nil {
		t.Errorf("expected nil for non-response event")
	}
}

func TestParseHijackEvent_NilResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventResponse, Response: nil}
	results := parseHijackEvent(ev, nil)
	if results != nil {
		t.Errorf("expected nil for nil response")
	}
}

func TestParseHijackEvent_EmptyBody(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://x.com/i/api/search/adaptive.json", Body: ""},
	}
	results := parseHijackEvent(ev, nil)
	if results != nil {
		t.Errorf("expected nil for empty body")
	}
}

func TestParseHijackEvent_UnknownEndpoint(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://x.com/i/api/unknown", Body: `{}`},
	}
	results := parseHijackEvent(ev, nil)
	if results != nil {
		t.Errorf("expected nil for unknown endpoint")
	}
}
