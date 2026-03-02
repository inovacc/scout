// Package linkedin implements the scraper.Mode interface for LinkedIn profile and post extraction.
// It intercepts LinkedIn's internal Voyager API calls via session hijacking to capture structured
// profile, post, connection, and job data without DOM scraping.
package linkedin

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// linkedinProvider implements auth.Provider for LinkedIn.
type linkedinProvider struct{}

func (p *linkedinProvider) Name() string { return "linkedin" }

func (p *linkedinProvider) LoginURL() string { return "https://www.linkedin.com/login" }

func (p *linkedinProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("linkedin: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("linkedin: detect auth: eval url: %w", err)
	}

	url := result.String()
	// Check for authenticated page indicators.
	if strings.Contains(url, "linkedin.com/feed") || strings.Contains(url, "linkedin.com/in/") {
		return true, nil
	}

	// Check for global-nav element which indicates logged-in state.
	_, err = page.Element(".global-nav")
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (p *linkedinProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("linkedin: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("linkedin: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("linkedin: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract CSRF token and other auth-related tokens from DOM/window.
	tokenResult, err := page.Eval(`() => {
		try {
			const csrfToken = document.querySelector('[data-csrf-token]');
			if (csrfToken) return csrfToken.getAttribute('data-csrf-token');
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		tok := tokenResult.String()
		if tok != "" {
			tokens["csrf_token"] = tok
		}
	}

	// Try to extract from window object or sessionStorage.
	sessionResult, err := page.Eval(`() => {
		try {
			if (window.__REACT_DEVTOOLS_GLOBAL_HOOK__ && window.__REACT_DEVTOOLS_GLOBAL_HOOK__.auth) {
				return JSON.stringify(window.__REACT_DEVTOOLS_GLOBAL_HOOK__.auth);
			}
			const session = sessionStorage.getItem('JSESSIONID');
			if (session) return session;
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		val := sessionResult.String()
		if val != "" {
			localStorage["session_data"] = val
		}
	}

	// Capture localStorage entries that may contain auth state.
	lsResult, err := page.Eval(`() => {
		try {
			const keys = Object.keys(localStorage);
			const result = {};
			for (const key of keys) {
				if (key.includes('auth') || key.includes('token') || key.includes('session')) {
					try {
						result[key] = localStorage.getItem(key);
					} catch(e) {}
				}
			}
			return JSON.stringify(result);
		} catch(e) {}
		return '{}';
	}`)
	if err == nil {
		raw := lsResult.String()
		if raw != "" && raw != "{}" {
			var lsMap map[string]string
			if json.Unmarshal([]byte(raw), &lsMap) == nil {
				maps.Copy(localStorage, lsMap)
			}
		}
	}

	now := time.Now()

	return &auth.Session{
		Provider:     "linkedin",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

func (p *linkedinProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("linkedin: validate session: nil session")
	}

	// Check for at least one key LinkedIn authentication cookie: li_at, JSESSIONID, or li_mc.
	for _, cookie := range session.Cookies {
		if cookie.Name == "li_at" || cookie.Name == "JSESSIONID" || cookie.Name == "li_mc" {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "no valid linkedin authentication cookies (li_at, JSESSIONID, or li_mc) found in session"}
}

// LinkedInMode implements scraper.Mode for LinkedIn.
type LinkedInMode struct {
	provider linkedinProvider
}

func (m *LinkedInMode) Name() string { return "linkedin" }
func (m *LinkedInMode) Description() string {
	return "Scrape LinkedIn profiles, posts, connections, and jobs"
}
func (m *LinkedInMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to LinkedIn,
// and intercepts Voyager API calls to extract structured data.
func (m *LinkedInMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	linkedinSession, ok := session.(*auth.Session)
	if !ok || linkedinSession == nil {
		return nil, fmt.Errorf("linkedin: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, linkedinSession); err != nil {
		return nil, fmt.Errorf("linkedin: scrape: %w", err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	browser, err := scout.New(
		scout.WithHeadless(opts.Headless),
		scout.WithStealth(),
	)
	if err != nil {
		return nil, fmt.Errorf("linkedin: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(linkedinSession.URL)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("linkedin: scrape: new page: %w", err)
	}

	if err := page.SetCookies(linkedinSession.Cookies...); err != nil {
		browser.Close()
		return nil, fmt.Errorf("linkedin: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		browser.Close()
		return nil, fmt.Errorf("linkedin: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		browser.Close()
		return nil, fmt.Errorf("linkedin: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*linkedin.com/voyager/api/*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("linkedin: scrape: create hijacker: %w", err)
	}

	results := make(chan scraper.Result, 256)
	targetSet := buildTargetSet(opts.Targets)

	go func() {
		defer close(results)
		defer hijacker.Stop()
		defer browser.Close()

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		count := 0

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-hijacker.Events():
				if !ok {
					return
				}

				if opts.Limit > 0 && count >= opts.Limit {
					return
				}

				items := parseHijackEvent(ev, targetSet)
				for _, item := range items {
					select {
					case <-ctx.Done():
						return
					case results <- item:
						count++
						if opts.Limit > 0 && count >= opts.Limit {
							return
						}

						if opts.Progress != nil {
							opts.Progress(scraper.Progress{
								Phase:   "scraping",
								Current: count,
								Total:   opts.Limit,
								Message: fmt.Sprintf("captured %d items", count),
							})
						}
					}
				}
			}
		}
	}()

	return results, nil
}

// buildTargetSet creates a lookup set from target URLs or identifiers.
// An empty set means no filtering (capture all).
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		// Normalize by removing trailing slashes and protocol.
		normalized := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(t, "https://"), "/"))
		set[normalized] = struct{}{}
	}

	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from LinkedIn Voyager API responses.
func parseHijackEvent(ev scout.HijackEvent, targetSet map[string]struct{}) []scraper.Result {
	if ev.Type != scout.HijackEventResponse || ev.Response == nil {
		return nil
	}

	url := ev.Response.URL

	body := ev.Response.Body
	if body == "" {
		return nil
	}

	switch {
	case strings.Contains(url, "/voyager/api/identity/profiles/"):
		return parseProfileResponse(body, targetSet)
	case strings.Contains(url, "/voyager/api/feed/updates"):
		return parseFeedResponse(body, targetSet)
	case strings.Contains(url, "/voyager/api/connections"):
		return parseConnectionsResponse(body, targetSet)
	case strings.Contains(url, "/voyager/api/jobs/"):
		return parseJobsResponse(body, targetSet)
	case strings.Contains(url, "/voyager/api/messaging/conversations"):
		return parseMessagingResponse(body, targetSet)
	default:
		return nil
	}
}

// linkedinAPIResponse is a wrapper for LinkedIn Voyager API responses.
type linkedinAPIResponse struct {
	Data     json.RawMessage   `json:"data"`
	Elements []json.RawMessage `json:"elements"`
	Included []json.RawMessage `json:"included"`
	Status   int               `json:"status"`
	Errors   []map[string]any  `json:"errors"`
}

// profileResponse represents LinkedIn profile data from Voyager API.
type profileResponse struct {
	FirstName                string `json:"firstName"`
	LastName                 string `json:"lastName"`
	Headline                 string `json:"headline"`
	ProfilePictureDisplayURL string `json:"profilePictureDisplayUrl"`
	Location                 string `json:"location"`
	Industry                 string `json:"industry"`
	PublicIdentifier         string `json:"publicIdentifier"`
	EntityURN                string `json:"entityUrn"`
	CreatedAt                int64  `json:"createdAt"`
	PublicProfileURL         string `json:"publicProfileUrl"`
	DerivedLocation          string `json:"derivedLocation"`
	OpenToWork               bool   `json:"openToWork"`
	PremiumSubscriber        bool   `json:"premiumSubscriber"`
}

func parseProfileResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp linkedinAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	if len(resp.Data) == 0 {
		return nil
	}

	var profile profileResponse

	if err := json.Unmarshal(resp.Data, &profile); err != nil {
		return nil
	}

	// Filter by target set if provided.
	if targetSet != nil {
		profileID := strings.ToLower(profile.PublicIdentifier)
		if _, ok := targetSet[profileID]; !ok {
			return nil
		}
	}

	ts := parseLinkedInTimestamp(profile.CreatedAt)
	result := scraper.Result{
		Type:      scraper.ResultProfile,
		Source:    "linkedin",
		ID:        profile.EntityURN,
		Timestamp: ts,
		Author:    profile.FirstName + " " + profile.LastName,
		Content:   profile.Headline,
		URL:       profile.PublicProfileURL,
		Metadata: map[string]any{
			"first_name":   profile.FirstName,
			"last_name":    profile.LastName,
			"location":     profile.Location,
			"industry":     profile.Industry,
			"public_id":    profile.PublicIdentifier,
			"open_to_work": profile.OpenToWork,
			"premium":      profile.PremiumSubscriber,
			"derived_loc":  profile.DerivedLocation,
		},
		Raw: profile,
	}

	return []scraper.Result{result}
}

// postResponse represents a post/update from LinkedIn feed.
type postResponse struct {
	ActivityID    string `json:"id"`
	Commentary    string `json:"commentary"`
	CreatedTime   int64  `json:"createdTime"`
	Actor         string `json:"actor"`
	ObjectUrn     string `json:"objectUrn"`
	ReactionCount int    `json:"reactionCount"`
}

func parseFeedResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp linkedinAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Process elements (individual posts).
	for _, elemRaw := range resp.Elements {
		var post postResponse

		if err := json.Unmarshal(elemRaw, &post); err != nil {
			continue
		}

		if post.ActivityID == "" {
			continue
		}

		ts := parseLinkedInTimestamp(post.CreatedTime)
		result := scraper.Result{
			Type:      scraper.ResultPost,
			Source:    "linkedin",
			ID:        post.ActivityID,
			Timestamp: ts,
			Author:    post.Actor,
			Content:   post.Commentary,
			Metadata: map[string]any{
				"object_urn":     post.ObjectUrn,
				"reaction_count": post.ReactionCount,
			},
			Raw: post,
		}

		results = append(results, result)
	}

	return results
}

// connectionResponse represents a connection/member from LinkedIn.
type connectionResponse struct {
	EntityURN         string `json:"entityUrn"`
	PublicIdentifier  string `json:"publicIdentifier"`
	FirstName         string `json:"firstName"`
	LastName          string `json:"lastName"`
	Headline          string `json:"headline"`
	ProfilePictureURL string `json:"profilePictureUrl"`
	Location          string `json:"location"`
	ConnectionDegree  string `json:"connectionDegree"`
	CreatedTime       int64  `json:"createdTime"`
}

func parseConnectionsResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp linkedinAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	for _, elemRaw := range resp.Elements {
		var conn connectionResponse

		if err := json.Unmarshal(elemRaw, &conn); err != nil {
			continue
		}

		if conn.EntityURN == "" {
			continue
		}

		ts := parseLinkedInTimestamp(conn.CreatedTime)
		result := scraper.Result{
			Type:      scraper.ResultMember,
			Source:    "linkedin",
			ID:        conn.EntityURN,
			Timestamp: ts,
			Author:    conn.FirstName + " " + conn.LastName,
			Content:   conn.Headline,
			Metadata: map[string]any{
				"public_id":         conn.PublicIdentifier,
				"location":          conn.Location,
				"connection_degree": conn.ConnectionDegree,
			},
			Raw: conn,
		}

		results = append(results, result)
	}

	return results
}

// jobResponse represents a job posting from LinkedIn.
type jobResponse struct {
	EntityURN       string `json:"entityUrn"`
	JobID           string `json:"jobID"`
	Title           string `json:"title"`
	CompanyName     string `json:"companyName"`
	Location        string `json:"location"`
	Description     string `json:"description"`
	PostedDate      int64  `json:"postedDate"`
	ApplyURL        string `json:"applyUrl"`
	ExperienceLevel string `json:"experienceLevel"`
}

func parseJobsResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp linkedinAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	for _, elemRaw := range resp.Elements {
		var job jobResponse

		if err := json.Unmarshal(elemRaw, &job); err != nil {
			continue
		}

		if job.EntityURN == "" {
			continue
		}

		ts := parseLinkedInTimestamp(job.PostedDate)
		result := scraper.Result{
			Type:      scraper.ResultMeeting, // Using as a placeholder; could be custom
			Source:    "linkedin",
			ID:        job.EntityURN,
			Timestamp: ts,
			Author:    job.CompanyName,
			Content:   job.Title + ": " + job.Description,
			URL:       job.ApplyURL,
			Metadata: map[string]any{
				"job_id":           job.JobID,
				"company":          job.CompanyName,
				"location":         job.Location,
				"experience_level": job.ExperienceLevel,
			},
			Raw: job,
		}

		results = append(results, result)
	}

	return results
}

// messageResponse represents a message/conversation from LinkedIn.
type messageResponse struct {
	ConversationID string `json:"conversationId"`
	ParticipantID  string `json:"participantId"`
	Subject        string `json:"subject"`
	CreatedTime    int64  `json:"createdTime"`
	LastMessageAt  int64  `json:"lastMessageAt"`
	Messages       []struct {
		MessageID   string `json:"messageId"`
		Content     string `json:"content"`
		CreatedTime int64  `json:"createdTime"`
		SenderID    string `json:"senderId"`
	} `json:"messages"`
}

func parseMessagingResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp linkedinAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	for _, elemRaw := range resp.Elements {
		var msg messageResponse

		if err := json.Unmarshal(elemRaw, &msg); err != nil {
			continue
		}

		if msg.ConversationID == "" {
			continue
		}

		ts := parseLinkedInTimestamp(msg.CreatedTime)
		result := scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    "linkedin",
			ID:        msg.ConversationID,
			Timestamp: ts,
			Author:    msg.ParticipantID,
			Content:   msg.Subject,
			Metadata: map[string]any{
				"participant_id":  msg.ParticipantID,
				"last_message_at": msg.LastMessageAt,
				"message_count":   len(msg.Messages),
			},
			Raw: msg,
		}

		results = append(results, result)

		// Emit individual messages as separate results.
		for _, m := range msg.Messages {
			msgTS := parseLinkedInTimestamp(m.CreatedTime)
			results = append(results, scraper.Result{
				Type:      scraper.ResultMessage,
				Source:    "linkedin",
				ID:        m.MessageID,
				Timestamp: msgTS,
				Author:    m.SenderID,
				Content:   m.Content,
				Metadata: map[string]any{
					"conversation_id": msg.ConversationID,
				},
			})
		}
	}

	return results
}

// parseLinkedInTimestamp converts a LinkedIn epoch timestamp (milliseconds) to time.Time.
func parseLinkedInTimestamp(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	// LinkedIn timestamps are typically in milliseconds.
	return time.Unix(0, ms*int64(time.Millisecond))
}

func init() {
	scraper.RegisterMode(&LinkedInMode{})
}
