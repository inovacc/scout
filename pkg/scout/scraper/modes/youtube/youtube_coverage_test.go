package youtube

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
)

// --- parseVideoPageResults tests (the /next comment parser) ---

func TestParseVideoPageResults(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantCount   int
		wantAuthor  string
		wantContent string
	}{
		{
			name: "single comment with multi-run content",
			body: `{
				"onResponseReceivedEndpoints": [
					{
						"appendContinuationItemsAction": {
							"continuationItems": [
								{
									"commentThreadRenderer": {
										"comment": {
											"commentRenderer": {
												"authorText": {"simpleText": "Alice"},
												"contentText": {"runs": [{"text": "Hello "}, {"text": "world"}]},
												"publishedTimeText": {"simpleText": "2 days ago"}
											}
										}
									}
								}
							]
						}
					}
				]
			}`,
			wantCount:   1,
			wantAuthor:  "Alice",
			wantContent: "Hello world",
		},
		{
			name: "multiple comments across endpoints",
			body: `{
				"onResponseReceivedEndpoints": [
					{
						"appendContinuationItemsAction": {
							"continuationItems": [
								{"commentThreadRenderer": {"comment": {"commentRenderer": {"authorText": {"simpleText": "Bob"}, "contentText": {"runs": [{"text": "First"}]}}}}},
								{"commentThreadRenderer": {"comment": {"commentRenderer": {"authorText": {"simpleText": "Carol"}, "contentText": {"runs": [{"text": "Second"}]}}}}}
							]
						}
					}
				]
			}`,
			wantCount: 2,
		},
		{
			name: "comment with empty author is skipped",
			body: `{
				"onResponseReceivedEndpoints": [
					{
						"appendContinuationItemsAction": {
							"continuationItems": [
								{"commentThreadRenderer": {"comment": {"commentRenderer": {"authorText": {"simpleText": ""}, "contentText": {"runs": [{"text": "orphan"}]}}}}}
							]
						}
					}
				]
			}`,
			wantCount: 0,
		},
		{
			name: "continuation item that is not a comment thread is ignored",
			body: `{
				"onResponseReceivedEndpoints": [
					{
						"appendContinuationItemsAction": {
							"continuationItems": [
								{"continuationItemRenderer": {"trigger": "CONTINUATION_TRIGGER_ON_ITEM_SHOWN"}}
							]
						}
					}
				]
			}`,
			wantCount: 0,
		},
		{
			name:      "no endpoints",
			body:      `{}`,
			wantCount: 0,
		},
		{
			name:      "invalid JSON returns nil",
			body:      `not json`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := parseVideoPageResults(tt.body, nil)
			if len(results) != tt.wantCount {
				t.Fatalf("got %d results, want %d", len(results), tt.wantCount)
			}
			if tt.wantCount == 0 {
				return
			}
			r := results[0]
			if r.Type != scraper.ResultComment {
				t.Errorf("Type = %q, want %q", r.Type, scraper.ResultComment)
			}
			if r.Source != "youtube" {
				t.Errorf("Source = %q, want %q", r.Source, "youtube")
			}
			if tt.wantAuthor != "" && r.Author != tt.wantAuthor {
				t.Errorf("Author = %q, want %q", r.Author, tt.wantAuthor)
			}
			if tt.wantContent != "" && r.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", r.Content, tt.wantContent)
			}
			if r.Metadata == nil {
				t.Error("Metadata is nil, want non-nil map")
			}
		})
	}
}

// --- parseSearchResults playlistRenderer branch ---

func TestParseSearchResults_PlaylistBranch(t *testing.T) {
	body := `{
		"contents": {
			"twoColumnSearchResultsRenderer": {
				"primaryContents": {
					"sectionListRenderer": {
						"contents": [
							{
								"itemSectionRenderer": {
									"contents": [
										{"playlistRenderer": {"playlistId": "PLsearch", "title": {"simpleText": "Search Playlist"}, "subtitle": {"simpleText": "5 videos"}}}
									]
								}
							}
						]
					}
				}
			}
		}
	}`

	results := parseSearchResults(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultChannel)
	}
	if results[0].ID != "PLsearch" {
		t.Errorf("ID = %q, want %q", results[0].ID, "PLsearch")
	}
	if results[0].URL != "https://www.youtube.com/playlist?list=PLsearch" {
		t.Errorf("URL = %q", results[0].URL)
	}
}

func TestParseSearchResults_UnknownRendererIgnored(t *testing.T) {
	// An item-section content with no recognized renderer yields no results.
	body := `{
		"contents": {
			"twoColumnSearchResultsRenderer": {
				"primaryContents": {
					"sectionListRenderer": {
						"contents": [
							{"itemSectionRenderer": {"contents": [{"shelfRenderer": {"title": "Shelf"}}]}}
						]
					}
				}
			}
		}
	}`

	results := parseSearchResults(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for unknown renderer, got %d", len(results))
	}
}

// --- parseBrowseResults playlistRenderer else-branch ---

func TestParseBrowseResults_PlaylistBranch(t *testing.T) {
	body := `{
		"contents": {
			"twoColumnBrowseResultsRenderer": {
				"tabs": [
					{
						"tabRenderer": {
							"content": {
								"sectionListRenderer": {
									"contents": [
										{
											"itemSectionRenderer": {
												"contents": [
													{"playlistRenderer": {"playlistId": "PLbrowse", "title": {"simpleText": "Browse Playlist"}, "subtitle": {"simpleText": "12 videos"}}}
												]
											}
										}
									]
								}
							}
						}
					}
				]
			}
		}
	}`

	results := parseBrowseResults(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultChannel)
	}
	if results[0].ID != "PLbrowse" {
		t.Errorf("ID = %q, want %q", results[0].ID, "PLbrowse")
	}
}

func TestParseBrowseResults_EmptyContents(t *testing.T) {
	results := parseBrowseResults(`{}`, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- parseHijackEvent switch-branch coverage (browse / next / player) ---

func TestParseHijackEvent_Branches(t *testing.T) {
	searchBody := `{
		"contents": {
			"twoColumnSearchResultsRenderer": {
				"primaryContents": {
					"sectionListRenderer": {
						"contents": [
							{"itemSectionRenderer": {"contents": [
								{"videoRenderer": {"videoId": "vs1", "title": {"runs": [{"text": "SearchVid"}]}}}
							]}}
						]
					}
				}
			}
		}
	}`

	browseBody := `{
		"contents": {
			"twoColumnBrowseResultsRenderer": {
				"tabs": [
					{"tabRenderer": {"content": {"sectionListRenderer": {"contents": [
						{"itemSectionRenderer": {"contents": [
							{"videoRenderer": {"videoId": "vb1", "title": {"runs": [{"text": "BrowseVid"}]}}}
						]}}
					]}}}}
				]
			}
		}
	}`

	nextBody := `{
		"onResponseReceivedEndpoints": [
			{"appendContinuationItemsAction": {"continuationItems": [
				{"commentThreadRenderer": {"comment": {"commentRenderer": {"authorText": {"simpleText": "Dave"}, "contentText": {"runs": [{"text": "nice"}]}}}}}
			]}}
		]
	}`

	tests := []struct {
		name      string
		url       string
		body      string
		wantCount int
		wantType  scraper.ResultType
	}{
		{
			name:      "search branch",
			url:       "https://www.youtube.com/youtubei/v1/search?key=x",
			body:      searchBody,
			wantCount: 1,
			wantType:  scraper.ResultPost,
		},
		{
			name:      "browse branch",
			url:       "https://www.youtube.com/youtubei/v1/browse?key=x",
			body:      browseBody,
			wantCount: 1,
			wantType:  scraper.ResultPost,
		},
		{
			name:      "next branch",
			url:       "https://www.youtube.com/youtubei/v1/next?key=x",
			body:      nextBody,
			wantCount: 1,
			wantType:  scraper.ResultComment,
		},
		{
			name:      "player branch returns nil",
			url:       "https://www.youtube.com/youtubei/v1/player?key=x",
			body:      `{"videoDetails": {"videoId": "p1"}}`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := scout.HijackEvent{
				Type:     scout.HijackEventResponse,
				Response: &scout.CapturedResponse{URL: tt.url, Body: tt.body},
			}
			results := parseHijackEvent(ev, nil)
			if len(results) != tt.wantCount {
				t.Fatalf("got %d results, want %d", len(results), tt.wantCount)
			}
			if tt.wantCount > 0 && results[0].Type != tt.wantType {
				t.Errorf("Type = %q, want %q", results[0].Type, tt.wantType)
			}
		})
	}
}

func TestParseHijackEvent_NilResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventResponse, Response: nil}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil response, got %d", len(results))
	}
}
