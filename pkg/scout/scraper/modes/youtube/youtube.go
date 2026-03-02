// Package youtube implements the scraper.Mode interface for YouTube extraction.
// It intercepts YouTube's InnerTube API calls via session hijacking to capture structured
// video, comment, channel, and playlist data without DOM scraping.
package youtube

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

// youtubeProvider implements auth.Provider for YouTube accounts.
type youtubeProvider struct{}

func (p *youtubeProvider) Name() string { return "youtube" }

func (p *youtubeProvider) LoginURL() string { return "https://accounts.google.com/" }

func (p *youtubeProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("youtube: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("youtube: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, "youtube.com") {
		// Check for avatar button indicating logged-in state.
		_, err = page.Element("#avatar-btn")
		if err == nil {
			return true, nil
		}

		// Alternative: check for account button.
		_, err = page.Element("button[aria-label='Account']")
		if err == nil {
			return true, nil
		}
	}

	return false, nil
}

func (p *youtubeProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("youtube: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("youtube: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("youtube: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract localStorage data related to YouTube session.
	lsResult, err := page.Eval(`() => {
		try {
			const keys = Object.keys(localStorage);
			const data = {};
			for (const key of keys) {
				if (key.includes('youtube') || key.includes('google') || key.includes('session')) {
					data[key] = localStorage.getItem(key);
				}
			}
			return JSON.stringify(data);
		} catch(e) {}
		return '{}';
	}`)
	if err == nil {
		raw := lsResult.String()
		if raw != "" && raw != "{}" {
			var lsData map[string]string
			if json.Unmarshal([]byte(raw), &lsData) == nil {
				maps.Copy(localStorage, lsData)
			}
		}
	}

	// Extract SAPISID and other tokens from sessionStorage and window objects.
	tokenResult, err := page.Eval(`() => {
		const tokens = {};
		try {
			// Check for SAPISID token (used for InnerTube API).
			if (window.__INNERTUBE_API_KEY__) {
				tokens.innertube_api_key = window.__INNERTUBE_API_KEY__;
			}
			if (window.__INNERTUBE_CLIENT_VERSION__) {
				tokens.innertube_client_version = window.__INNERTUBE_CLIENT_VERSION__;
			}
			// Try to get authorization token from sessionStorage.
			for (const key of Object.keys(sessionStorage)) {
				if (key.includes('token') || key.includes('auth')) {
					tokens[key] = sessionStorage.getItem(key);
				}
			}
		} catch(e) {}
		return JSON.stringify(tokens);
	}`)
	if err == nil {
		raw := tokenResult.String()
		if raw != "" && raw != "{}" {
			var tokensData map[string]string
			if json.Unmarshal([]byte(raw), &tokensData) == nil {
				maps.Copy(tokens, tokensData)
			}
		}
	}

	now := time.Now()

	return &auth.Session{
		Provider:     "youtube",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

func (p *youtubeProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("youtube: validate session: nil session")
	}

	// Check for essential YouTube/Google cookies (SID, SSID, HSID, LOGIN_INFO).
	requiredCookies := map[string]bool{}

	for _, cookie := range session.Cookies {
		switch cookie.Name {
		case "SID", "SSID", "HSID", "LOGIN_INFO":
			requiredCookies[cookie.Name] = true
		}
	}

	if len(requiredCookies) == 0 {
		return &scraper.AuthError{Reason: "no valid youtube/google cookies (SID/SSID/HSID/LOGIN_INFO) found in session"}
	}

	return nil
}

// YouTubeMode implements scraper.Mode for YouTube.
type YouTubeMode struct {
	provider youtubeProvider
}

func (m *YouTubeMode) Name() string { return "youtube" }
func (m *YouTubeMode) Description() string {
	return "Scrape YouTube videos, comments, channels, and playlists"
}
func (m *YouTubeMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to YouTube,
// and intercepts InnerTube API calls to extract structured data.
func (m *YouTubeMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	youtubeSession, ok := session.(*auth.Session)
	if !ok || youtubeSession == nil {
		return nil, fmt.Errorf("youtube: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, youtubeSession); err != nil {
		return nil, fmt.Errorf("youtube: scrape: %w", err)
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
		return nil, fmt.Errorf("youtube: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage("https://www.youtube.com")
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("youtube: scrape: new page: %w", err)
	}

	if err := page.SetCookies(youtubeSession.Cookies...); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("youtube: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("youtube: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("youtube: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*/youtube.com/youtubei/*", "*/youtube.com/api/*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("youtube: scrape: create hijacker: %w", err)
	}

	results := make(chan scraper.Result, 256)
	targetSet := buildTargetSet(opts.Targets)

	go func() {
		defer close(results)
		defer hijacker.Stop()
		defer func() { _ = browser.Close() }()

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

// buildTargetSet creates a lookup set from target URLs or channel IDs. An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		// Normalize by lowercasing and trimming URL schemes.
		normalized := strings.ToLower(strings.TrimSpace(t))
		set[normalized] = struct{}{}
	}

	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from YouTube API responses.
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
	case strings.Contains(url, "youtubei/v1/search"):
		return parseSearchResults(body, targetSet)
	case strings.Contains(url, "youtubei/v1/browse"):
		return parseBrowseResults(body, targetSet)
	case strings.Contains(url, "youtubei/v1/next"):
		return parseVideoPageResults(body, targetSet)
	case strings.Contains(url, "youtubei/v1/player"):
		return parsePlayerResults(body, targetSet)
	default:
		return nil
	}
}

// youtubeAPIResponse is the common envelope for YouTube InnerTube API responses.
type youtubeAPIResponse struct {
	ResponseContext struct {
		ServiceTrackingParams []map[string]any `json:"serviceTrackingParams"`
	} `json:"responseContext"`
}

type searchResponse struct {
	youtubeAPIResponse

	Contents struct {
		TwoColumnSearchResultsRenderer struct {
			PrimaryContents struct {
				SectionListRenderer struct {
					Contents []struct {
						ItemSectionRenderer struct {
							Contents []json.RawMessage `json:"contents"`
						} `json:"itemSectionRenderer"`
					} `json:"contents"`
				} `json:"sectionListRenderer"`
			} `json:"primaryContents"`
		} `json:"twoColumnSearchResultsRenderer"`
	} `json:"contents"`
}

type browseResponse struct {
	youtubeAPIResponse

	Contents struct {
		TwoColumnBrowseResultsRenderer struct {
			Tabs []struct {
				TabRenderer struct {
					Content struct {
						SectionListRenderer struct {
							Contents []struct {
								ItemSectionRenderer struct {
									Contents []json.RawMessage `json:"contents"`
								} `json:"itemSectionRenderer"`
							} `json:"contents"`
						} `json:"sectionListRenderer"`
					} `json:"content"`
				} `json:"tabRenderer"`
			} `json:"tabs"`
		} `json:"twoColumnBrowseResultsRenderer"`
	} `json:"contents"`
}

type videoRenderer struct {
	VideoID string `json:"videoId"`
	Title   struct {
		Runs []struct {
			Text string `json:"text"`
		} `json:"runs"`
	} `json:"title"`
	ShortBylineText struct {
		SimpleText string `json:"simpleText"`
	} `json:"shortBylineText"`
	ViewCountText struct {
		SimpleText string `json:"simpleText"`
	} `json:"viewCountText"`
	PublishedTimeText struct {
		SimpleText string `json:"simpleText"`
	} `json:"publishedTimeText"`
}

type channelRenderer struct {
	ChannelID string `json:"channelId"`
	Title     struct {
		SimpleText string `json:"simpleText"`
	} `json:"title"`
	DescriptionSnippet struct {
		RichText struct {
			Content string `json:"content"`
		} `json:"richText"`
	} `json:"descriptionSnippet"`
	SubscriberCountText struct {
		SimpleText string `json:"simpleText"`
	} `json:"subscriberCountText"`
}

type playlistRenderer struct {
	PlaylistID string `json:"playlistId"`
	Title      struct {
		SimpleText string `json:"simpleText"`
	} `json:"title"`
	Subtitle struct {
		SimpleText string `json:"simpleText"`
	} `json:"subtitle"`
}

func parseSearchResults(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp searchResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Navigate the nested structure to find video renderers.
	if resp.Contents.TwoColumnSearchResultsRenderer.PrimaryContents.SectionListRenderer.Contents != nil {
		for _, section := range resp.Contents.TwoColumnSearchResultsRenderer.PrimaryContents.SectionListRenderer.Contents {
			if section.ItemSectionRenderer.Contents != nil {
				for _, content := range section.ItemSectionRenderer.Contents {
					// Try to parse as video renderer.
					var videoData struct {
						VideoRenderer    *videoRenderer    `json:"videoRenderer"`
						ChannelRenderer  *channelRenderer  `json:"channelRenderer"`
						PlaylistRenderer *playlistRenderer `json:"playlistRenderer"`
					}
					if json.Unmarshal(content, &videoData) == nil {
						switch {
						case videoData.VideoRenderer != nil:
							results = append(results, videoRendererToResult(videoData.VideoRenderer))
						case videoData.ChannelRenderer != nil:
							results = append(results, channelRendererToResult(videoData.ChannelRenderer))
						case videoData.PlaylistRenderer != nil:
							results = append(results, playlistRendererToResult(videoData.PlaylistRenderer))
						}
					}
				}
			}
		}
	}

	return results
}

func parseBrowseResults(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp browseResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Navigate the nested structure to find video renderers.
	if resp.Contents.TwoColumnBrowseResultsRenderer.Tabs != nil {
		for _, tab := range resp.Contents.TwoColumnBrowseResultsRenderer.Tabs {
			if tab.TabRenderer.Content.SectionListRenderer.Contents != nil {
				for _, section := range tab.TabRenderer.Content.SectionListRenderer.Contents {
					if section.ItemSectionRenderer.Contents != nil {
						for _, content := range section.ItemSectionRenderer.Contents {
							// Try to parse as video renderer.
							var videoData struct {
								VideoRenderer    *videoRenderer    `json:"videoRenderer"`
								PlaylistRenderer *playlistRenderer `json:"playlistRenderer"`
							}
							if json.Unmarshal(content, &videoData) == nil {
								if videoData.VideoRenderer != nil {
									results = append(results, videoRendererToResult(videoData.VideoRenderer))
								} else if videoData.PlaylistRenderer != nil {
									results = append(results, playlistRendererToResult(videoData.PlaylistRenderer))
								}
							}
						}
					}
				}
			}
		}
	}

	return results
}

func parseVideoPageResults(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	// The /next endpoint returns comments and related videos.
	// This is a simplified parser; a full implementation would extract more metadata.
	var resp struct {
		OnResponseReceivedEndpoints []struct {
			AppendContinuationItemsAction struct {
				ContinuationItems []json.RawMessage `json:"continuationItems"`
			} `json:"appendContinuationItemsAction"`
		} `json:"onResponseReceivedEndpoints"`
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Parse comments and related videos from continuation items.
	for _, endpoint := range resp.OnResponseReceivedEndpoints {
		for _, item := range endpoint.AppendContinuationItemsAction.ContinuationItems {
			// Try to parse as comment thread renderer.
			var commentData struct {
				CommentThreadRenderer struct {
					Comment struct {
						CommentRenderer struct {
							AuthorText struct {
								SimpleText string `json:"simpleText"`
							} `json:"authorText"`
							ContentText struct {
								Runs []struct {
									Text string `json:"text"`
								} `json:"runs"`
							} `json:"contentText"`
							PublishedTimeText struct {
								SimpleText string `json:"simpleText"`
							} `json:"publishedTimeText"`
						} `json:"commentRenderer"`
					} `json:"comment"`
				} `json:"commentThreadRenderer"`
			}
			if json.Unmarshal(item, &commentData) == nil && commentData.CommentThreadRenderer.Comment.CommentRenderer.AuthorText.SimpleText != "" {
				var contentText strings.Builder
				for _, run := range commentData.CommentThreadRenderer.Comment.CommentRenderer.ContentText.Runs {
					contentText.WriteString(run.Text)
				}

				results = append(results, scraper.Result{
					Type:      scraper.ResultComment,
					Source:    "youtube",
					ID:        fmt.Sprintf("%s-%d", commentData.CommentThreadRenderer.Comment.CommentRenderer.AuthorText.SimpleText, time.Now().UnixNano()),
					Timestamp: time.Now(),
					Author:    commentData.CommentThreadRenderer.Comment.CommentRenderer.AuthorText.SimpleText,
					Content:   contentText.String(),
					Metadata:  make(map[string]any),
					Raw:       commentData,
				})
			}
		}
	}

	return results
}

func parsePlayerResults(body string, targetSet map[string]struct{}) []scraper.Result {
	// The /player endpoint returns video metadata and availability info.
	// This is where you'd extract detailed video info if needed.
	return nil
}

func videoRendererToResult(v *videoRenderer) scraper.Result {
	title := ""
	if len(v.Title.Runs) > 0 {
		title = v.Title.Runs[0].Text
	}

	author := v.ShortBylineText.SimpleText

	viewCount := v.ViewCountText.SimpleText
	publishedTime := v.PublishedTimeText.SimpleText

	return scraper.Result{
		Type:      scraper.ResultPost,
		Source:    "youtube",
		ID:        v.VideoID,
		Timestamp: time.Now(),
		Author:    author,
		Content:   title,
		URL:       "https://www.youtube.com/watch?v=" + v.VideoID,
		Metadata: map[string]any{
			"title":          title,
			"view_count":     viewCount,
			"published_time": publishedTime,
			"video_id":       v.VideoID,
		},
		Raw: v,
	}
}

func channelRendererToResult(c *channelRenderer) scraper.Result {
	title := c.Title.SimpleText
	description := c.DescriptionSnippet.RichText.Content
	subscriberCount := c.SubscriberCountText.SimpleText

	return scraper.Result{
		Type:      scraper.ResultProfile,
		Source:    "youtube",
		ID:        c.ChannelID,
		Timestamp: time.Now(),
		Author:    title,
		Content:   description,
		URL:       "https://www.youtube.com/channel/" + c.ChannelID,
		Metadata: map[string]any{
			"title":            title,
			"channel_id":       c.ChannelID,
			"subscriber_count": subscriberCount,
		},
		Raw: c,
	}
}

func playlistRendererToResult(p *playlistRenderer) scraper.Result {
	title := p.Title.SimpleText
	subtitle := p.Subtitle.SimpleText

	return scraper.Result{
		Type:      scraper.ResultChannel,
		Source:    "youtube",
		ID:        p.PlaylistID,
		Timestamp: time.Now(),
		Content:   title,
		URL:       "https://www.youtube.com/playlist?list=" + p.PlaylistID,
		Metadata: map[string]any{
			"title":       title,
			"subtitle":    subtitle,
			"playlist_id": p.PlaylistID,
		},
		Raw: p,
	}
}

func init() {
	scraper.RegisterMode(&YouTubeMode{})
}
