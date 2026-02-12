package slack

import (
	"context"
	"fmt"

	"github.com/inovacc/scout/scraper"
)

// Scraper extracts data from a Slack workspace using the web API.
type Scraper struct {
	opts      *options
	api       *apiClient
	workspace Workspace
	creds     scraper.Credentials
}

// New creates a new Slack scraper with the given options.
func New(opts ...Option) *Scraper {
	o := defaults()
	for _, fn := range opts {
		fn(o)
	}

	return &Scraper{opts: o}
}

// Authenticate validates or acquires credentials for the Slack workspace.
// If a token and d-cookie are provided, it validates them via auth.test.
// Otherwise, it opens a browser for interactive login.
func (s *Scraper) Authenticate(ctx context.Context) error {
	if s.opts.token != "" && s.opts.dCookie != "" {
		return s.authenticateWithToken(ctx)
	}

	return s.authenticateWithBrowser(ctx)
}

// GetWorkspace returns the authenticated workspace metadata.
func (s *Scraper) GetWorkspace() Workspace {
	return s.workspace
}

// GetCredentials returns the current authentication credentials.
func (s *Scraper) GetCredentials() scraper.Credentials {
	return s.creds
}

// ListChannels returns all channels visible to the authenticated user.
func (s *Scraper) ListChannels(ctx context.Context) ([]Channel, error) {
	if s.api == nil {
		return nil, &scraper.AuthError{Reason: "not authenticated"}
	}

	var channels []Channel

	cursor := ""

	for {
		s.reportProgress("channels", len(channels), 0, "fetching channels")

		resp, err := s.api.conversationsList(ctx, cursor)
		if err != nil {
			return nil, fmt.Errorf("slack: list channels: %w", err)
		}

		for _, cj := range resp.Channels {
			channels = append(channels, toChannel(cj))
		}

		cursor = resp.Meta.NextCursor
		if cursor == "" {
			break
		}
	}

	s.reportProgress("channels", len(channels), len(channels), "done")

	return channels, nil
}

// GetMessages returns messages from a channel, respecting maxMessages and date range options.
func (s *Scraper) GetMessages(ctx context.Context, channelID string) ([]Message, error) {
	if s.api == nil {
		return nil, &scraper.AuthError{Reason: "not authenticated"}
	}

	var messages []Message

	cursor := ""
	limit := 100

	for {
		if s.opts.maxMessages > 0 {
			remaining := s.opts.maxMessages - len(messages)
			if remaining <= 0 {
				break
			}

			if remaining < limit {
				limit = remaining
			}
		}

		s.reportProgress("messages", len(messages), s.opts.maxMessages, fmt.Sprintf("fetching messages from %s", channelID))

		resp, err := s.api.conversationsHistory(ctx, channelID, cursor, s.opts.oldest, s.opts.latest, limit)
		if err != nil {
			return nil, fmt.Errorf("slack: get messages %s: %w", channelID, err)
		}

		for _, mj := range resp.Messages {
			messages = append(messages, toMessage(mj))
		}

		if !resp.HasMore {
			break
		}

		cursor = resp.Meta.NextCursor
		if cursor == "" {
			break
		}

		if s.opts.maxMessages > 0 && len(messages) >= s.opts.maxMessages {
			break
		}
	}

	if s.opts.maxMessages > 0 && len(messages) > s.opts.maxMessages {
		messages = messages[:s.opts.maxMessages]
	}

	s.reportProgress("messages", len(messages), len(messages), "done")

	return messages, nil
}

// GetThreadReplies returns all replies in a thread.
func (s *Scraper) GetThreadReplies(ctx context.Context, channelID, threadTS string) ([]Message, error) {
	if s.api == nil {
		return nil, &scraper.AuthError{Reason: "not authenticated"}
	}

	var messages []Message

	cursor := ""

	for {
		resp, err := s.api.conversationsReplies(ctx, channelID, threadTS, cursor)
		if err != nil {
			return nil, fmt.Errorf("slack: get thread replies %s/%s: %w", channelID, threadTS, err)
		}

		for _, mj := range resp.Messages {
			messages = append(messages, toMessage(mj))
		}

		if !resp.HasMore {
			break
		}

		cursor = resp.Meta.NextCursor
		if cursor == "" {
			break
		}
	}

	return messages, nil
}

// ListUsers returns all members of the workspace.
func (s *Scraper) ListUsers(ctx context.Context) ([]User, error) {
	if s.api == nil {
		return nil, &scraper.AuthError{Reason: "not authenticated"}
	}

	var users []User

	cursor := ""

	for {
		s.reportProgress("users", len(users), 0, "fetching users")

		resp, err := s.api.usersList(ctx, cursor)
		if err != nil {
			return nil, fmt.Errorf("slack: list users: %w", err)
		}

		for _, uj := range resp.Members {
			users = append(users, toUser(uj))
		}

		cursor = resp.Meta.NextCursor
		if cursor == "" {
			break
		}
	}

	s.reportProgress("users", len(users), len(users), "done")

	return users, nil
}

// ListFiles returns files shared in a channel (or all files if channelID is empty).
func (s *Scraper) ListFiles(ctx context.Context, channelID string) ([]File, error) {
	if s.api == nil {
		return nil, &scraper.AuthError{Reason: "not authenticated"}
	}

	var files []File

	page := 1

	for {
		s.reportProgress("files", len(files), 0, "fetching files")

		resp, err := s.api.filesList(ctx, channelID, page)
		if err != nil {
			return nil, fmt.Errorf("slack: list files: %w", err)
		}

		for _, fj := range resp.Files {
			files = append(files, toFile(fj))
		}

		if page >= resp.Paging.Pages {
			break
		}

		page++
	}

	s.reportProgress("files", len(files), len(files), "done")

	return files, nil
}

// Search searches messages matching the query.
func (s *Scraper) Search(ctx context.Context, query string) ([]SearchResult, error) {
	if s.api == nil {
		return nil, &scraper.AuthError{Reason: "not authenticated"}
	}

	s.reportProgress("search", 0, 0, fmt.Sprintf("searching: %s", query))

	resp, err := s.api.searchMessages(ctx, query, 1)
	if err != nil {
		return nil, fmt.Errorf("slack: search %q: %w", query, err)
	}

	results := make([]SearchResult, 0, len(resp.Messages.Matches))
	for _, mj := range resp.Messages.Matches {
		results = append(results, toSearchResult(mj))
	}

	s.reportProgress("search", len(results), resp.Messages.Total, "done")

	return results, nil
}

// ExportChannel exports a full channel: metadata, messages, and optionally threads.
func (s *Scraper) ExportChannel(ctx context.Context, channelID string) (*ChannelExport, error) {
	if s.api == nil {
		return nil, &scraper.AuthError{Reason: "not authenticated"}
	}

	// Find the channel metadata
	channels, err := s.ListChannels(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack: export channel: %w", err)
	}

	var channel Channel

	found := false

	for _, ch := range channels {
		if ch.ID == channelID {
			channel = ch
			found = true

			break
		}
	}

	if !found {
		return nil, fmt.Errorf("slack: export channel: channel %s not found", channelID)
	}

	// Get messages
	messages, err := s.GetMessages(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("slack: export channel: %w", err)
	}

	export := &ChannelExport{
		Channel:  channel,
		Messages: messages,
	}

	// Optionally fetch threads
	if s.opts.includeThreads {
		for i, msg := range messages {
			if msg.ReplyCount > 0 && msg.ThreadTS != "" {
				s.reportProgress("threads", i, len(messages), fmt.Sprintf("fetching thread %s", msg.ThreadTS))

				replies, err := s.GetThreadReplies(ctx, channelID, msg.ThreadTS)
				if err != nil {
					return nil, fmt.Errorf("slack: export channel: %w", err)
				}

				// First message in replies is the parent; skip it
				threadReplies := replies
				if len(replies) > 1 {
					threadReplies = replies[1:]
				}

				export.Threads = append(export.Threads, Thread{
					Parent:  msg,
					Replies: threadReplies,
				})
			}
		}
	}

	return export, nil
}

func (s *Scraper) reportProgress(phase string, current, total int, message string) {
	if s.opts.progress != nil {
		s.opts.progress(phase, current, total, message)
	}
}
