package slack

// Workspace holds metadata about a Slack workspace.
type Workspace struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
	URL    string `json:"url"`
}

// Channel represents a Slack channel (public, private, DM, or group DM).
type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Topic      string `json:"topic"`
	Purpose    string `json:"purpose"`
	MemberCount int   `json:"member_count"`
	IsPrivate  bool   `json:"is_private"`
	IsArchived bool   `json:"is_archived"`
	IsIM       bool   `json:"is_im"`
	IsMPIM     bool   `json:"is_mpim"`
	Created    int64  `json:"created"`
}

// Message represents a single Slack message.
type Message struct {
	Type      string     `json:"type"`
	User      string     `json:"user"`
	Text      string     `json:"text"`
	Timestamp string     `json:"ts"`
	ThreadTS  string     `json:"thread_ts,omitempty"`
	ReplyCount int       `json:"reply_count,omitempty"`
	Reactions []Reaction `json:"reactions,omitempty"`
	Files     []File     `json:"files,omitempty"`
	Edited    *Edited    `json:"edited,omitempty"`
	SubType   string     `json:"subtype,omitempty"`
	BotID     string     `json:"bot_id,omitempty"`
}

// Thread represents a message thread with its replies.
type Thread struct {
	Parent  Message   `json:"parent"`
	Replies []Message `json:"replies"`
}

// User represents a Slack workspace member.
type User struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	IsBot       bool   `json:"is_bot"`
	IsAdmin     bool   `json:"is_admin"`
	IsOwner     bool   `json:"is_owner"`
	Deleted     bool   `json:"deleted"`
	TimeZone    string `json:"tz,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
}

// File represents a file shared in Slack.
type File struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Title     string `json:"title"`
	MimeType  string `json:"mimetype"`
	Size      int64  `json:"size"`
	URL       string `json:"url_private"`
	Permalink string `json:"permalink"`
	User      string `json:"user"`
	Created   int64  `json:"created"`
}

// Reaction represents an emoji reaction on a message.
type Reaction struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

// Edited indicates a message was edited.
type Edited struct {
	User      string `json:"user"`
	Timestamp string `json:"ts"`
}

// SearchResult represents a single search hit.
type SearchResult struct {
	Channel   string  `json:"channel"`
	User      string  `json:"user"`
	Text      string  `json:"text"`
	Timestamp string  `json:"ts"`
	Permalink string  `json:"permalink"`
	Score     float64 `json:"score,omitempty"`
}

// ChannelExport contains a full channel export with metadata, messages, and optionally threads.
type ChannelExport struct {
	Channel  Channel   `json:"channel"`
	Messages []Message `json:"messages"`
	Threads  []Thread  `json:"threads,omitempty"`
}
