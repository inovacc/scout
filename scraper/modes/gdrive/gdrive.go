// Package gdrive implements the scraper.Mode interface for Google Drive extraction.
// It intercepts Google Drive's internal API calls via session hijacking to capture structured
// file, folder, sharing, and comment data without DOM scraping.
package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/scraper"
	"github.com/inovacc/scout/scraper/auth"
)

// gdriveProvider implements auth.Provider for Google Drive.
type gdriveProvider struct{}

func (p *gdriveProvider) Name() string { return "gdrive" }

func (p *gdriveProvider) LoginURL() string { return "https://accounts.google.com/" }

func (p *gdriveProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("gdrive: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("gdrive: detect auth: eval url: %w", err)
	}

	url := result.String()
	if !strings.Contains(url, "drive.google.com") {
		return false, nil
	}

	// Check for Google Drive specific elements.
	_, err = page.Element("[data-tooltip=\"My Drive\"]")
	if err == nil {
		return true, nil
	}

	// Alternative selector for My Drive element.
	_, err = page.Element(".a-Ja-c")
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (p *gdriveProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("gdrive: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("gdrive: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("gdrive: capture session: eval url: %w", err)
	}
	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Capture localStorage for authentication tokens and state.
	lsResult, err := page.Eval(`() => {
		try {
			const keys = Object.keys(localStorage);
			const result = {};
			for (const key of keys) {
				if (key.includes('token') || key.includes('auth') || key.includes('goog')) {
					result[key] = localStorage.getItem(key);
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
				for k, v := range lsMap {
					localStorage[k] = v
					// Extract any bearer tokens.
					if strings.Contains(v, "Bearer ") || strings.HasPrefix(v, "ya29") {
						tokens[k] = v
					}
				}
			}
		}
	}

	now := time.Now()
	return &auth.Session{
		Provider:     "gdrive",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

func (p *gdriveProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("gdrive: validate session: nil session")
	}

	// Check for essential Google Drive authentication cookies.
	foundSID := false
	foundSSID := false
	for _, cookie := range session.Cookies {
		if cookie.Name == "SID" {
			foundSID = true
		}
		if cookie.Name == "SSID" {
			foundSSID = true
		}
	}

	if !foundSID && !foundSSID {
		return &scraper.AuthError{Reason: "no valid google auth cookies (SID/SSID) found in session"}
	}

	return nil
}

// GDriveMode implements scraper.Mode for Google Drive.
type GDriveMode struct {
	provider gdriveProvider
}

func (m *GDriveMode) Name() string { return "gdrive" }
func (m *GDriveMode) Description() string {
	return "Scrape Google Drive files, folders, sharing, and comments"
}
func (m *GDriveMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to Google Drive,
// and intercepts Google Drive API calls to extract structured data.
func (m *GDriveMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	gdriveSession, ok := session.(*auth.Session)
	if !ok || gdriveSession == nil {
		return nil, fmt.Errorf("gdrive: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, gdriveSession); err != nil {
		return nil, fmt.Errorf("gdrive: scrape: %w", err)
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
		return nil, fmt.Errorf("gdrive: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage("https://drive.google.com/drive/my-drive")
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("gdrive: scrape: new page: %w", err)
	}

	if err := page.SetCookies(gdriveSession.Cookies...); err != nil {
		browser.Close()
		return nil, fmt.Errorf("gdrive: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		browser.Close()
		return nil, fmt.Errorf("gdrive: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		browser.Close()
		return nil, fmt.Errorf("gdrive: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*drive.google.com*", "*clients6.google.com*", "*content.googleapis.com*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("gdrive: scrape: create hijacker: %w", err)
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

// buildTargetSet creates a lookup set from target folder IDs or shared drive names.
// An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		set[strings.ToLower(t)] = struct{}{}
	}
	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from Google Drive API responses.
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
	case strings.Contains(url, "files/list") || strings.Contains(url, "drive_api/files/list"):
		return parseFilesList(body, targetSet)
	case strings.Contains(url, "files/get") || strings.Contains(url, "drive_api/files/get"):
		return parseFileGet(body)
	case strings.Contains(url, "folders") || strings.Contains(url, "teamdrives"):
		return parseFolderList(body, targetSet)
	case strings.Contains(url, "permissions") || strings.Contains(url, "sharedbody"):
		return parsePermissions(body)
	case strings.Contains(url, "comments") || strings.Contains(url, "replies"):
		return parseComments(body)
	default:
		return nil
	}
}

// parseFilesList handles Google Drive files.list API responses.
func parseFilesList(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp struct {
		Files []gdFile `json:"files"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Files))
	for _, f := range resp.Files {
		// Filter by parent folder if targets specified.
		if targetSet != nil && len(f.Parents) > 0 {
			found := false
			for _, parent := range f.Parents {
				if _, ok := targetSet[strings.ToLower(parent)]; ok {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		ts := parseGoogleTimestamp(f.CreatedTime)
		var size int64
		if f.Size != "" {
			fmt.Sscanf(f.Size, "%d", &size)
		}

		result := scraper.Result{
			Type:      scraper.ResultFile,
			Source:    "gdrive",
			ID:        f.ID,
			Timestamp: ts,
			Content:   f.Name,
			Metadata: map[string]any{
				"name":          f.Name,
				"mime_type":     f.MimeType,
				"size":          size,
				"owners":        f.Owners,
				"last_modified": f.ModifiedTime,
				"web_view_link": f.WebViewLink,
			},
			Raw: f,
		}
		if len(f.Owners) > 0 {
			result.Author = f.Owners[0]
		}
		results = append(results, result)
	}
	return results
}

// parseFileGet handles individual file metadata responses.
func parseFileGet(body string) []scraper.Result {
	var f gdFile
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		return nil
	}

	if f.ID == "" {
		return nil
	}

	ts := parseGoogleTimestamp(f.CreatedTime)
	var size int64
	if f.Size != "" {
		fmt.Sscanf(f.Size, "%d", &size)
	}

	result := scraper.Result{
		Type:      scraper.ResultFile,
		Source:    "gdrive",
		ID:        f.ID,
		Timestamp: ts,
		Content:   f.Name,
		Metadata: map[string]any{
			"name":          f.Name,
			"mime_type":     f.MimeType,
			"size":          size,
			"owners":        f.Owners,
			"last_modified": f.ModifiedTime,
			"web_view_link": f.WebViewLink,
		},
		Raw: f,
	}
	if len(f.Owners) > 0 {
		result.Author = f.Owners[0]
	}
	return []scraper.Result{result}
}

// parseFolderList handles folder/shared drive listing responses.
func parseFolderList(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp struct {
		TeamDrives []gdFolder `json:"teamDrives,omitempty"`
		Drives     []gdFolder `json:"drives,omitempty"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	folders := append(resp.TeamDrives, resp.Drives...)
	results := make([]scraper.Result, 0, len(folders))

	for _, f := range folders {
		if targetSet != nil {
			if _, ok := targetSet[strings.ToLower(f.Name)]; !ok {
				continue
			}
		}

		ts := parseGoogleTimestamp(f.CreatedTime)
		results = append(results, scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "gdrive",
			ID:        f.ID,
			Timestamp: ts,
			Content:   f.Name,
			Metadata: map[string]any{
				"name":              f.Name,
				"organization_name": f.OrganizationName,
			},
			Raw: f,
		})
	}
	return results
}

// parsePermissions handles sharing/permissions responses.
func parsePermissions(body string) []scraper.Result {
	var resp struct {
		Permissions []gdPermission `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Permissions))
	for _, p := range resp.Permissions {
		ts := parseGoogleTimestamp(p.CreatedTime)
		results = append(results, scraper.Result{
			Type:      scraper.ResultMember,
			Source:    "gdrive",
			ID:        p.ID,
			Timestamp: ts,
			Author:    p.EmailAddress,
			Content:   p.Role,
			Metadata: map[string]any{
				"email":        p.EmailAddress,
				"role":         p.Role,
				"type":         p.Type,
				"display_name": p.DisplayName,
			},
			Raw: p,
		})
	}
	return results
}

// parseComments handles comment and reply responses.
func parseComments(body string) []scraper.Result {
	var resp struct {
		Comments []gdComment `json:"comments"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Comments))
	for _, c := range resp.Comments {
		ts := parseGoogleTimestamp(c.CreatedTime)
		result := scraper.Result{
			Type:      scraper.ResultComment,
			Source:    "gdrive",
			ID:        c.ID,
			Timestamp: ts,
			Content:   c.Content,
			Metadata: map[string]any{
				"file_id":       c.FileID,
				"author_email":  c.Author.EmailAddress,
				"author_name":   c.Author.DisplayName,
				"resolved":      c.Resolved,
				"modified_time": c.ModifiedTime,
			},
			Raw: c,
		}
		if c.Author.DisplayName != "" {
			result.Author = c.Author.DisplayName
		} else if c.Author.EmailAddress != "" {
			result.Author = c.Author.EmailAddress
		}
		results = append(results, result)
	}
	return results
}

// Google Drive API response types.

type gdFile struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	MimeType     string   `json:"mimeType"`
	Size         string   `json:"size"`
	Parents      []string `json:"parents"`
	Owners       []string `json:"owners"`
	CreatedTime  string   `json:"createdTime"`
	ModifiedTime string   `json:"modifiedTime"`
	WebViewLink  string   `json:"webViewLink"`
}

type gdFolder struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	CreatedTime      string `json:"createdTime"`
	OrganizationName string `json:"organizationName"`
}

type gdPermission struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
	CreatedTime  string `json:"createdTime"`
}

type gdComment struct {
	ID           string `json:"id"`
	FileID       string `json:"fileId"`
	Content      string `json:"content"`
	CreatedTime  string `json:"createdTime"`
	ModifiedTime string `json:"modifiedTime"`
	Resolved     bool   `json:"resolved"`
	Author       struct {
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
	} `json:"author"`
}

// parseGoogleTimestamp converts an RFC3339 timestamp string to time.Time.
func parseGoogleTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}

	// Google Drive uses RFC3339 format (e.g. "2024-02-28T10:30:45.123Z").
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}

func init() {
	scraper.RegisterMode(&GDriveMode{})
}
