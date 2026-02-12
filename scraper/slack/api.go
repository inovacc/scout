package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/scout/scraper"
)

const slackAPIBase = "https://slack.com/api"

// apiClient makes authenticated requests to the Slack web API.
type apiClient struct {
	baseURL    string
	token      string
	dCookie    string
	httpClient *http.Client
	rateLimit  time.Duration
	mu         sync.Mutex
	lastCall   time.Time
}

func newAPIClient(token, dCookie string, rateLimit time.Duration) *apiClient {
	return &apiClient{
		baseURL:    slackAPIBase,
		token:      token,
		dCookie:    dCookie,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		rateLimit:  rateLimit,
	}
}

// slackResponse is the base response envelope for all Slack API calls.
type slackResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// channelsResponse is the response from conversations.list.
type channelsResponse struct {
	slackResponse

	Channels []channelJSON    `json:"channels"`
	Meta     responseMetadata `json:"response_metadata"`
}

// historyResponse is the response from conversations.history.
type historyResponse struct {
	slackResponse

	Messages []messageJSON    `json:"messages"`
	HasMore  bool             `json:"has_more"`
	Meta     responseMetadata `json:"response_metadata"`
}

// repliesResponse is the response from conversations.replies.
type repliesResponse struct {
	slackResponse

	Messages []messageJSON    `json:"messages"`
	HasMore  bool             `json:"has_more"`
	Meta     responseMetadata `json:"response_metadata"`
}

// usersResponse is the response from users.list.
type usersResponse struct {
	slackResponse

	Members []userJSON       `json:"members"`
	Meta    responseMetadata `json:"response_metadata"`
}

// filesResponse is the response from files.list.
type filesResponse struct {
	slackResponse

	Files  []fileJSON `json:"files"`
	Paging paging     `json:"paging"`
}

// searchResponse is the response from search.messages.
type searchResponse struct {
	slackResponse

	Messages searchMessages `json:"messages"`
}

// authTestResponse is the response from auth.test.
type authTestResponse struct {
	slackResponse

	URL    string `json:"url"`
	Team   string `json:"team"`
	TeamID string `json:"team_id"`
	User   string `json:"user"`
	UserID string `json:"user_id"`
}

type responseMetadata struct {
	NextCursor string `json:"next_cursor"`
}

type paging struct {
	Page  int `json:"page"`
	Pages int `json:"pages"`
	Total int `json:"total"`
}

type searchMessages struct {
	Matches []searchMatchJSON `json:"matches"`
	Total   int               `json:"total"`
	Paging  paging            `json:"paging"`
}

// channelJSON maps the JSON structure from Slack's conversations.list.
type channelJSON struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Topic      topicJSON `json:"topic"`
	Purpose    topicJSON `json:"purpose"`
	NumMembers int       `json:"num_members"`
	IsPrivate  bool      `json:"is_private"`
	IsArchived bool      `json:"is_archived"`
	IsIM       bool      `json:"is_im"`
	IsMPIM     bool      `json:"is_mpim"`
	Created    int64     `json:"created"`
}

type topicJSON struct {
	Value string `json:"value"`
}

// messageJSON maps the JSON structure from Slack's message responses.
type messageJSON struct {
	Type       string         `json:"type"`
	User       string         `json:"user"`
	Text       string         `json:"text"`
	TS         string         `json:"ts"`
	ThreadTS   string         `json:"thread_ts,omitempty"`
	ReplyCount int            `json:"reply_count,omitempty"`
	Reactions  []reactionJSON `json:"reactions,omitempty"`
	Files      []fileJSON     `json:"files,omitempty"`
	Edited     *editedJSON    `json:"edited,omitempty"`
	SubType    string         `json:"subtype,omitempty"`
	BotID      string         `json:"bot_id,omitempty"`
}

type reactionJSON struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

type editedJSON struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

// userJSON maps the JSON structure from Slack's users.list.
type userJSON struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	RealName string      `json:"real_name"`
	Profile  profileJSON `json:"profile"`
	IsBot    bool        `json:"is_bot"`
	IsAdmin  bool        `json:"is_admin"`
	IsOwner  bool        `json:"is_owner"`
	Deleted  bool        `json:"deleted"`
	TZ       string      `json:"tz"`
}

type profileJSON struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Image72     string `json:"image_72"`
}

// fileJSON maps the JSON structure from Slack's files.list.
type fileJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Title     string `json:"title"`
	MimeType  string `json:"mimetype"`
	Size      int64  `json:"size"`
	URLPriv   string `json:"url_private"`
	Permalink string `json:"permalink"`
	User      string `json:"user"`
	Created   int64  `json:"created"`
}

// searchMatchJSON maps search result matches.
type searchMatchJSON struct {
	Channel   searchChannelJSON `json:"channel"`
	User      string            `json:"user"`
	Username  string            `json:"username"`
	Text      string            `json:"text"`
	TS        string            `json:"ts"`
	Permalink string            `json:"permalink"`
	Score     float64           `json:"score,omitempty"`
}

type searchChannelJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// post makes an authenticated POST request to a Slack API method.
func (c *apiClient) post(ctx context.Context, method string, params url.Values) ([]byte, error) {
	c.waitRateLimit()

	if params == nil {
		params = url.Values{}
	}

	params.Set("token", c.token)

	reqURL := c.baseURL + "/" + method

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("slack: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if c.dCookie != "" {
		req.AddCookie(&http.Cookie{Name: "d", Value: c.dCookie})
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack: http request %s: %w", method, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("slack: read response %s: %w", method, err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := 0
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			retryAfter, _ = strconv.Atoi(ra)
		}

		return nil, &scraper.RateLimitError{RetryAfter: retryAfter}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("slack: %s: unexpected status %d", method, resp.StatusCode)
	}

	return body, nil
}

// checkResponse parses the base response and returns an error for Slack-level failures.
func checkResponse(body []byte) error {
	var base slackResponse
	if err := json.Unmarshal(body, &base); err != nil {
		return fmt.Errorf("slack: parse response: %w", err)
	}

	if !base.OK {
		switch base.Error {
		case "invalid_auth", "not_authed", "token_revoked", "token_expired", "account_inactive":
			return &scraper.AuthError{Reason: base.Error}
		case "ratelimited":
			return &scraper.RateLimitError{}
		default:
			return fmt.Errorf("slack: api error: %s", base.Error)
		}
	}

	return nil
}

// authTest calls auth.test to validate the token and return workspace info.
func (c *apiClient) authTest(ctx context.Context) (*authTestResponse, error) {
	body, err := c.post(ctx, "auth.test", nil)
	if err != nil {
		return nil, err
	}

	if err := checkResponse(body); err != nil {
		return nil, err
	}

	var resp authTestResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("slack: parse auth.test: %w", err)
	}

	return &resp, nil
}

// conversationsList calls conversations.list with cursor pagination.
func (c *apiClient) conversationsList(ctx context.Context, cursor string) (*channelsResponse, error) {
	params := url.Values{
		"types": {"public_channel,private_channel,mpim,im"},
		"limit": {"200"},
	}

	if cursor != "" {
		params.Set("cursor", cursor)
	}

	body, err := c.post(ctx, "conversations.list", params)
	if err != nil {
		return nil, err
	}

	if err := checkResponse(body); err != nil {
		return nil, err
	}

	var resp channelsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("slack: parse conversations.list: %w", err)
	}

	return &resp, nil
}

// conversationsHistory calls conversations.history with cursor pagination.
func (c *apiClient) conversationsHistory(ctx context.Context, channelID, cursor, oldest, latest string, limit int) (*historyResponse, error) {
	params := url.Values{
		"channel": {channelID},
		"limit":   {strconv.Itoa(limit)},
	}

	if cursor != "" {
		params.Set("cursor", cursor)
	}

	if oldest != "" {
		params.Set("oldest", oldest)
	}

	if latest != "" {
		params.Set("latest", latest)
	}

	body, err := c.post(ctx, "conversations.history", params)
	if err != nil {
		return nil, err
	}

	if err := checkResponse(body); err != nil {
		return nil, err
	}

	var resp historyResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("slack: parse conversations.history: %w", err)
	}

	return &resp, nil
}

// conversationsReplies calls conversations.replies for a thread.
func (c *apiClient) conversationsReplies(ctx context.Context, channelID, threadTS, cursor string) (*repliesResponse, error) {
	params := url.Values{
		"channel": {channelID},
		"ts":      {threadTS},
		"limit":   {"200"},
	}

	if cursor != "" {
		params.Set("cursor", cursor)
	}

	body, err := c.post(ctx, "conversations.replies", params)
	if err != nil {
		return nil, err
	}

	if err := checkResponse(body); err != nil {
		return nil, err
	}

	var resp repliesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("slack: parse conversations.replies: %w", err)
	}

	return &resp, nil
}

// usersList calls users.list with cursor pagination.
func (c *apiClient) usersList(ctx context.Context, cursor string) (*usersResponse, error) {
	params := url.Values{
		"limit": {"200"},
	}

	if cursor != "" {
		params.Set("cursor", cursor)
	}

	body, err := c.post(ctx, "users.list", params)
	if err != nil {
		return nil, err
	}

	if err := checkResponse(body); err != nil {
		return nil, err
	}

	var resp usersResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("slack: parse users.list: %w", err)
	}

	return &resp, nil
}

// filesList calls files.list with page-based pagination.
func (c *apiClient) filesList(ctx context.Context, channelID string, page int) (*filesResponse, error) {
	params := url.Values{
		"count": {"100"},
		"page":  {strconv.Itoa(page)},
	}

	if channelID != "" {
		params.Set("channel", channelID)
	}

	body, err := c.post(ctx, "files.list", params)
	if err != nil {
		return nil, err
	}

	if err := checkResponse(body); err != nil {
		return nil, err
	}

	var resp filesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("slack: parse files.list: %w", err)
	}

	return &resp, nil
}

// searchMessages calls search.messages.
func (c *apiClient) searchMessages(ctx context.Context, query string, page int) (*searchResponse, error) {
	params := url.Values{
		"query": {query},
		"count": {"20"},
		"page":  {strconv.Itoa(page)},
	}

	body, err := c.post(ctx, "search.messages", params)
	if err != nil {
		return nil, err
	}

	if err := checkResponse(body); err != nil {
		return nil, err
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("slack: parse search.messages: %w", err)
	}

	return &resp, nil
}

// waitRateLimit enforces the minimum delay between API calls.
func (c *apiClient) waitRateLimit() {
	if c.rateLimit <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	since := time.Since(c.lastCall)
	if since < c.rateLimit {
		time.Sleep(c.rateLimit - since)
	}

	c.lastCall = time.Now()
}

// toChannel converts a channelJSON to a Channel.
func toChannel(cj channelJSON) Channel {
	return Channel{
		ID:          cj.ID,
		Name:        cj.Name,
		Topic:       cj.Topic.Value,
		Purpose:     cj.Purpose.Value,
		MemberCount: cj.NumMembers,
		IsPrivate:   cj.IsPrivate,
		IsArchived:  cj.IsArchived,
		IsIM:        cj.IsIM,
		IsMPIM:      cj.IsMPIM,
		Created:     cj.Created,
	}
}

// toMessage converts a messageJSON to a Message.
func toMessage(mj messageJSON) Message {
	m := Message{
		Type:       mj.Type,
		User:       mj.User,
		Text:       mj.Text,
		Timestamp:  mj.TS,
		ThreadTS:   mj.ThreadTS,
		ReplyCount: mj.ReplyCount,
		SubType:    mj.SubType,
		BotID:      mj.BotID,
	}

	for _, rj := range mj.Reactions {
		m.Reactions = append(m.Reactions, Reaction(rj))
	}

	for _, fj := range mj.Files {
		m.Files = append(m.Files, toFile(fj))
	}

	if mj.Edited != nil {
		m.Edited = &Edited{User: mj.Edited.User, Timestamp: mj.Edited.TS}
	}

	return m
}

// toUser converts a userJSON to a User.
func toUser(uj userJSON) User {
	return User{
		ID:          uj.ID,
		Name:        uj.Name,
		RealName:    uj.RealName,
		DisplayName: uj.Profile.DisplayName,
		Email:       uj.Profile.Email,
		IsBot:       uj.IsBot,
		IsAdmin:     uj.IsAdmin,
		IsOwner:     uj.IsOwner,
		Deleted:     uj.Deleted,
		TimeZone:    uj.TZ,
		Avatar:      uj.Profile.Image72,
	}
}

// toFile converts a fileJSON to a File.
func toFile(fj fileJSON) File {
	return File{
		ID:        fj.ID,
		Name:      fj.Name,
		Title:     fj.Title,
		MimeType:  fj.MimeType,
		Size:      fj.Size,
		URL:       fj.URLPriv,
		Permalink: fj.Permalink,
		User:      fj.User,
		Created:   fj.Created,
	}
}

// toSearchResult converts a searchMatchJSON to a SearchResult.
func toSearchResult(mj searchMatchJSON) SearchResult {
	return SearchResult{
		Channel:   mj.Channel.ID,
		User:      mj.User,
		Text:      mj.Text,
		Timestamp: mj.TS,
		Permalink: mj.Permalink,
		Score:     mj.Score,
	}
}
