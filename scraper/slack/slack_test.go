package slack

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/inovacc/scout/scraper"
)

func newTestScraper(baseURL string, opts ...Option) *Scraper {
	allOpts := append([]Option{
		WithToken("xoxc-valid-token"),
		WithDCookie("xoxd-test"),
		WithRateLimit(0), // no delay in tests
	}, opts...)

	s := New(allOpts...)
	s.api = newAPIClient("xoxc-valid-token", "xoxd-test", 0)
	s.api.baseURL = baseURL

	return s
}

func TestScraper_NotAuthenticated(t *testing.T) {
	s := New()
	ctx := context.Background()

	var authErr *scraper.AuthError

	_, err := s.ListChannels(ctx)
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}

	_, err = s.GetMessages(ctx, "C01")
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError for GetMessages, got %T: %v", err, err)
	}

	_, err = s.GetThreadReplies(ctx, "C01", "123.456")
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError for GetThreadReplies, got %T: %v", err, err)
	}

	_, err = s.ListUsers(ctx)
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError for ListUsers, got %T: %v", err, err)
	}

	_, err = s.ListFiles(ctx, "C01")
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError for ListFiles, got %T: %v", err, err)
	}

	_, err = s.Search(ctx, "query")
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError for Search, got %T: %v", err, err)
	}

	_, err = s.ExportChannel(ctx, "C01")
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError for ExportChannel, got %T: %v", err, err)
	}
}

func TestScraper_ListChannels(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := newTestScraper(srv.URL)

	channels, err := s.ListChannels(context.Background())
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}

	// 2 from page 1 + 1 from page 2
	if len(channels) != 3 {
		t.Fatalf("channels = %d, want 3", len(channels))
	}

	if channels[0].Name != "general" {
		t.Fatalf("first channel = %q, want %q", channels[0].Name, "general")
	}

	if channels[0].MemberCount != 42 {
		t.Fatalf("member_count = %d, want 42", channels[0].MemberCount)
	}

	if !channels[1].IsPrivate {
		t.Fatal("second channel should be private")
	}

	if channels[2].Name != "random" {
		t.Fatalf("third channel = %q, want %q", channels[2].Name, "random")
	}
}

func TestScraper_GetMessages(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := newTestScraper(srv.URL)

	messages, err := s.GetMessages(context.Background(), "C01GENERAL")
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}

	// 2 from page 1 + 1 from page 2
	if len(messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(messages))
	}

	msg := messages[0]

	if msg.Text != "Hello world" {
		t.Fatalf("text = %q, want %q", msg.Text, "Hello world")
	}

	if len(msg.Reactions) != 1 {
		t.Fatalf("reactions = %d, want 1", len(msg.Reactions))
	}

	if msg.Reactions[0].Name != "thumbsup" {
		t.Fatalf("reaction = %q, want %q", msg.Reactions[0].Name, "thumbsup")
	}

	if messages[1].Edited == nil {
		t.Fatal("second message should have edited field")
	}
}

func TestScraper_GetMessages_MaxMessages(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := newTestScraper(srv.URL, WithMaxMessages(1))

	messages, err := s.GetMessages(context.Background(), "C01GENERAL")
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
}

func TestScraper_GetThreadReplies(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := newTestScraper(srv.URL)

	replies, err := s.GetThreadReplies(context.Background(), "C01GENERAL", "1600000003.000000")
	if err != nil {
		t.Fatalf("GetThreadReplies: %v", err)
	}

	if len(replies) != 3 {
		t.Fatalf("replies = %d, want 3", len(replies))
	}

	if replies[1].Text != "Reply 1" {
		t.Fatalf("reply text = %q, want %q", replies[1].Text, "Reply 1")
	}
}

func TestScraper_ListUsers(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := newTestScraper(srv.URL)

	users, err := s.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("users = %d, want 2", len(users))
	}

	if users[0].RealName != "Test User" {
		t.Fatalf("real_name = %q, want %q", users[0].RealName, "Test User")
	}

	if users[0].Email != "test@example.com" {
		t.Fatalf("email = %q, want %q", users[0].Email, "test@example.com")
	}

	if !users[1].IsBot {
		t.Fatal("second user should be a bot")
	}
}

func TestScraper_ListFiles(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := newTestScraper(srv.URL)

	files, err := s.ListFiles(context.Background(), "C01GENERAL")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}

	if files[0].Name != "report.pdf" {
		t.Fatalf("name = %q, want %q", files[0].Name, "report.pdf")
	}
}

func TestScraper_Search(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := newTestScraper(srv.URL)

	results, err := s.Search(context.Background(), "matching")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	if results[0].Channel != "C01GENERAL" {
		t.Fatalf("channel = %q, want %q", results[0].Channel, "C01GENERAL")
	}
}

func TestScraper_ExportChannel(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := newTestScraper(srv.URL, WithIncludeThreads(true))

	export, err := s.ExportChannel(context.Background(), "C01GENERAL")
	if err != nil {
		t.Fatalf("ExportChannel: %v", err)
	}

	if export.Channel.Name != "general" {
		t.Fatalf("channel = %q, want %q", export.Channel.Name, "general")
	}

	if len(export.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(export.Messages))
	}

	// One thread (the first message has reply_count > 0)
	if len(export.Threads) != 1 {
		t.Fatalf("threads = %d, want 1", len(export.Threads))
	}

	thread := export.Threads[0]

	if thread.Parent.Text != "Hello world" {
		t.Fatalf("thread parent = %q, want %q", thread.Parent.Text, "Hello world")
	}

	// Thread replies exclude the parent message
	if len(thread.Replies) != 2 {
		t.Fatalf("thread replies = %d, want 2", len(thread.Replies))
	}
}

func TestScraper_ExportChannel_NotFound(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := newTestScraper(srv.URL)

	_, err := s.ExportChannel(context.Background(), "CNOTFOUND")
	if err == nil {
		t.Fatal("expected error for non-existent channel")
	}
}

func TestScraper_Progress(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	var calls []string

	s := newTestScraper(srv.URL, WithProgress(func(phase string, _, _ int, _ string) {
		calls = append(calls, phase)
	}))

	_, _ = s.ListChannels(context.Background())

	if len(calls) == 0 {
		t.Fatal("expected progress callbacks")
	}

	if !slices.Contains(calls, "channels") {
		t.Fatalf("expected 'channels' phase in progress calls, got %v", calls)
	}
}

func TestScraper_GetWorkspace(t *testing.T) {
	s := New()
	s.workspace = Workspace{ID: "T01", Name: "test"}

	ws := s.GetWorkspace()
	if ws.ID != "T01" {
		t.Fatalf("workspace ID = %q, want %q", ws.ID, "T01")
	}
}

func TestScraper_GetCredentials(t *testing.T) {
	s := New()
	s.creds = scraper.Credentials{Token: "xoxc-test"}

	creds := s.GetCredentials()
	if creds.Token != "xoxc-test" {
		t.Fatalf("token = %q, want %q", creds.Token, "xoxc-test")
	}
}
