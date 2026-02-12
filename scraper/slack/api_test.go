package slack

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/inovacc/scout/scraper"
)

func TestAPIClient_AuthTest(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	client := newAPIClient("xoxc-valid-token", "xoxd-test", 0)
	client.baseURL = srv.URL

	resp, err := client.authTest(context.Background())
	if err != nil {
		t.Fatalf("authTest: %v", err)
	}

	if resp.Team != "Test Team" {
		t.Fatalf("team = %q, want %q", resp.Team, "Test Team")
	}

	if resp.TeamID != "T01TEST" {
		t.Fatalf("team_id = %q, want %q", resp.TeamID, "T01TEST")
	}

	if resp.User != "testuser" {
		t.Fatalf("user = %q, want %q", resp.User, "testuser")
	}
}

func TestAPIClient_AuthTest_InvalidToken(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	client := newAPIClient("xoxc-bad-token", "", 0)
	client.baseURL = srv.URL

	_, err := client.authTest(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid token")
	}

	var authErr *scraper.AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}
}

func TestAPIClient_ConversationsList(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	client := newAPIClient("xoxc-valid-token", "xoxd-test", 0)
	client.baseURL = srv.URL

	ctx := context.Background()

	// First page
	resp, err := client.conversationsList(ctx, "")
	if err != nil {
		t.Fatalf("conversationsList page 1: %v", err)
	}

	if len(resp.Channels) != 2 {
		t.Fatalf("channels = %d, want 2", len(resp.Channels))
	}

	if resp.Meta.NextCursor != "page2" {
		t.Fatalf("cursor = %q, want %q", resp.Meta.NextCursor, "page2")
	}

	// Second page
	resp, err = client.conversationsList(ctx, "page2")
	if err != nil {
		t.Fatalf("conversationsList page 2: %v", err)
	}

	if len(resp.Channels) != 1 {
		t.Fatalf("channels = %d, want 1", len(resp.Channels))
	}

	if resp.Meta.NextCursor != "" {
		t.Fatalf("cursor = %q, want empty", resp.Meta.NextCursor)
	}
}

func TestAPIClient_ConversationsHistory(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	client := newAPIClient("xoxc-valid-token", "xoxd-test", 0)
	client.baseURL = srv.URL

	resp, err := client.conversationsHistory(context.Background(), "C01GENERAL", "", "", "", 100)
	if err != nil {
		t.Fatalf("conversationsHistory: %v", err)
	}

	if len(resp.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(resp.Messages))
	}

	if !resp.HasMore {
		t.Fatal("expected has_more to be true")
	}

	msg := resp.Messages[0]

	if msg.Text != "Hello world" {
		t.Fatalf("text = %q, want %q", msg.Text, "Hello world")
	}

	if msg.ReplyCount != 2 {
		t.Fatalf("reply_count = %d, want 2", msg.ReplyCount)
	}

	if len(msg.Reactions) != 1 {
		t.Fatalf("reactions = %d, want 1", len(msg.Reactions))
	}

	msg2 := resp.Messages[1]
	if msg2.Edited == nil {
		t.Fatal("expected edited to be non-nil")
	}
}

func TestAPIClient_ConversationsReplies(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	client := newAPIClient("xoxc-valid-token", "xoxd-test", 0)
	client.baseURL = srv.URL

	resp, err := client.conversationsReplies(context.Background(), "C01GENERAL", "1600000003.000000", "")
	if err != nil {
		t.Fatalf("conversationsReplies: %v", err)
	}

	if len(resp.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(resp.Messages))
	}
}

func TestAPIClient_UsersList(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	client := newAPIClient("xoxc-valid-token", "xoxd-test", 0)
	client.baseURL = srv.URL

	resp, err := client.usersList(context.Background(), "")
	if err != nil {
		t.Fatalf("usersList: %v", err)
	}

	if len(resp.Members) != 2 {
		t.Fatalf("members = %d, want 2", len(resp.Members))
	}

	user := resp.Members[0]

	if user.RealName != "Test User" {
		t.Fatalf("real_name = %q, want %q", user.RealName, "Test User")
	}

	if user.Profile.Email != "test@example.com" {
		t.Fatalf("email = %q, want %q", user.Profile.Email, "test@example.com")
	}
}

func TestAPIClient_FilesList(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	client := newAPIClient("xoxc-valid-token", "xoxd-test", 0)
	client.baseURL = srv.URL

	resp, err := client.filesList(context.Background(), "C01GENERAL", 1)
	if err != nil {
		t.Fatalf("filesList: %v", err)
	}

	if len(resp.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(resp.Files))
	}

	f := resp.Files[0]

	if f.Name != "report.pdf" {
		t.Fatalf("name = %q, want %q", f.Name, "report.pdf")
	}

	if f.Size != 102400 {
		t.Fatalf("size = %d, want 102400", f.Size)
	}
}

func TestAPIClient_SearchMessages(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	client := newAPIClient("xoxc-valid-token", "xoxd-test", 0)
	client.baseURL = srv.URL

	resp, err := client.searchMessages(context.Background(), "matching", 1)
	if err != nil {
		t.Fatalf("searchMessages: %v", err)
	}

	if resp.Messages.Total != 1 {
		t.Fatalf("total = %d, want 1", resp.Messages.Total)
	}

	if len(resp.Messages.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(resp.Messages.Matches))
	}

	match := resp.Messages.Matches[0]
	if match.Channel.ID != "C01GENERAL" {
		t.Fatalf("channel = %q, want %q", match.Channel.ID, "C01GENERAL")
	}
}

func TestAPIClient_RateLimit(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	delay := 50 * time.Millisecond
	client := newAPIClient("xoxc-valid-token", "xoxd-test", delay)
	client.baseURL = srv.URL

	ctx := context.Background()
	start := time.Now()

	_, _ = client.authTest(ctx)
	_, _ = client.authTest(ctx)

	elapsed := time.Since(start)
	if elapsed < delay {
		t.Fatalf("expected at least %v between calls, got %v", delay, elapsed)
	}
}

func TestAPIClient_ContextCancellation(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	client := newAPIClient("xoxc-valid-token", "xoxd-test", 0)
	client.baseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.authTest(ctx)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestCheckResponse_UnknownError(t *testing.T) {
	body := []byte(`{"ok": false, "error": "some_unknown_error"}`)

	err := checkResponse(body)
	if err == nil {
		t.Fatal("expected error")
	}

	var authErr *scraper.AuthError
	if errors.As(err, &authErr) {
		t.Fatal("should not be AuthError for unknown error")
	}
}

func TestConversionFunctions(t *testing.T) {
	t.Run("toChannel", func(t *testing.T) {
		ch := toChannel(channelJSON{
			ID:         "C01",
			Name:       "general",
			Topic:      topicJSON{Value: "hello"},
			Purpose:    topicJSON{Value: "world"},
			NumMembers: 10,
			IsPrivate:  true,
		})

		if ch.ID != "C01" || ch.Name != "general" || ch.Topic != "hello" || !ch.IsPrivate {
			t.Fatalf("unexpected channel: %+v", ch)
		}
	})

	t.Run("toMessage", func(t *testing.T) {
		m := toMessage(messageJSON{
			Type: "message",
			User: "U01",
			Text: "hello",
			TS:   "123.456",
			Reactions: []reactionJSON{
				{Name: "thumbsup", Count: 1, Users: []string{"U01"}},
			},
			Edited: &editedJSON{User: "U01", TS: "123.789"},
		})

		if m.User != "U01" || len(m.Reactions) != 1 || m.Edited == nil {
			t.Fatalf("unexpected message: %+v", m)
		}
	})

	t.Run("toUser", func(t *testing.T) {
		u := toUser(userJSON{
			ID:       "U01",
			Name:     "test",
			RealName: "Test User",
			Profile:  profileJSON{DisplayName: "tester", Email: "t@e.com"},
			IsBot:    true,
		})

		if u.ID != "U01" || !u.IsBot || u.Email != "t@e.com" {
			t.Fatalf("unexpected user: %+v", u)
		}
	})

	t.Run("toFile", func(t *testing.T) {
		f := toFile(fileJSON{
			ID: "F01", Name: "test.txt", Size: 1024,
		})

		if f.ID != "F01" || f.Size != 1024 {
			t.Fatalf("unexpected file: %+v", f)
		}
	})

	t.Run("toSearchResult", func(t *testing.T) {
		sr := toSearchResult(searchMatchJSON{
			Channel: searchChannelJSON{ID: "C01"},
			User:    "U01",
			Text:    "match",
		})

		if sr.Channel != "C01" || sr.User != "U01" {
			t.Fatalf("unexpected search result: %+v", sr)
		}
	})
}
