package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

// newMockSlackAPI creates an httptest server that mimics Slack API endpoints.
func newMockSlackAPI() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/auth.test", handleAuthTest)
	mux.HandleFunc("/conversations.list", handleConversationsList)
	mux.HandleFunc("/conversations.history", handleConversationsHistory)
	mux.HandleFunc("/conversations.replies", handleConversationsReplies)
	mux.HandleFunc("/users.list", handleUsersList)
	mux.HandleFunc("/files.list", handleFilesList)
	mux.HandleFunc("/search.messages", handleSearchMessages)

	return httptest.NewServer(mux)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")

	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(b)
}

func checkToken(r *http.Request) bool {
	_ = r.ParseForm()

	token := r.FormValue("token")

	return token == "xoxc-valid-token"
}

func handleAuthTest(w http.ResponseWriter, r *http.Request) {
	if !checkToken(r) {
		writeJSON(w, map[string]any{"ok": false, "error": "invalid_auth"})
		return
	}

	writeJSON(w, map[string]any{
		"ok":      true,
		"url":     "https://testteam.slack.com/",
		"team":    "Test Team",
		"team_id": "T01TEST",
		"user":    "testuser",
		"user_id": "U01TEST",
	})
}

func handleConversationsList(w http.ResponseWriter, r *http.Request) {
	if !checkToken(r) {
		writeJSON(w, map[string]any{"ok": false, "error": "invalid_auth"})
		return
	}

	_ = r.ParseForm()

	cursor := r.FormValue("cursor")

	if cursor == "page2" {
		writeJSON(w, map[string]any{
			"ok": true,
			"channels": []map[string]any{
				{
					"id": "C03PAGE2", "name": "random",
					"topic": map[string]string{"value": "Random stuff"},
					"purpose": map[string]string{"value": "Random"},
					"num_members": 8, "is_private": false,
					"is_archived": false, "created": 1600000002,
				},
			},
			"response_metadata": map[string]string{"next_cursor": ""},
		})

		return
	}

	writeJSON(w, map[string]any{
		"ok": true,
		"channels": []map[string]any{
			{
				"id": "C01GENERAL", "name": "general",
				"topic":   map[string]string{"value": "Company-wide announcements"},
				"purpose": map[string]string{"value": "General discussion"},
				"num_members": 42, "is_private": false,
				"is_archived": false, "created": 1600000000,
			},
			{
				"id": "C02PRIVATE", "name": "secret-project",
				"topic":   map[string]string{"value": "Top secret"},
				"purpose": map[string]string{"value": "Secret stuff"},
				"num_members": 5, "is_private": true,
				"is_archived": false, "created": 1600000001,
			},
		},
		"response_metadata": map[string]string{"next_cursor": "page2"},
	})
}

func handleConversationsHistory(w http.ResponseWriter, r *http.Request) {
	if !checkToken(r) {
		writeJSON(w, map[string]any{"ok": false, "error": "invalid_auth"})
		return
	}

	_ = r.ParseForm()

	cursor := r.FormValue("cursor")

	if cursor == "page2" {
		writeJSON(w, map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{
					"type": "message", "user": "U01TEST",
					"text": "Older message", "ts": "1600000001.000000",
				},
			},
			"has_more":          false,
			"response_metadata": map[string]string{"next_cursor": ""},
		})

		return
	}

	writeJSON(w, map[string]any{
		"ok": true,
		"messages": []map[string]any{
			{
				"type": "message", "user": "U01TEST",
				"text": "Hello world", "ts": "1600000003.000000",
				"thread_ts": "1600000003.000000", "reply_count": 2,
				"reactions": []map[string]any{
					{"name": "thumbsup", "count": 3, "users": []string{"U01", "U02", "U03"}},
				},
			},
			{
				"type": "message", "user": "U02OTHER",
				"text": "Hi there", "ts": "1600000002.000000",
				"edited": map[string]string{"user": "U02OTHER", "ts": "1600000002.500000"},
			},
		},
		"has_more":          true,
		"response_metadata": map[string]string{"next_cursor": "page2"},
	})
}

func handleConversationsReplies(w http.ResponseWriter, r *http.Request) {
	if !checkToken(r) {
		writeJSON(w, map[string]any{"ok": false, "error": "invalid_auth"})
		return
	}

	writeJSON(w, map[string]any{
		"ok": true,
		"messages": []map[string]any{
			{
				"type": "message", "user": "U01TEST",
				"text": "Hello world", "ts": "1600000003.000000",
				"thread_ts": "1600000003.000000",
			},
			{
				"type": "message", "user": "U02OTHER",
				"text": "Reply 1", "ts": "1600000003.000100",
				"thread_ts": "1600000003.000000",
			},
			{
				"type": "message", "user": "U01TEST",
				"text": "Reply 2", "ts": "1600000003.000200",
				"thread_ts": "1600000003.000000",
			},
		},
		"has_more":          false,
		"response_metadata": map[string]string{"next_cursor": ""},
	})
}

func handleUsersList(w http.ResponseWriter, r *http.Request) {
	if !checkToken(r) {
		writeJSON(w, map[string]any{"ok": false, "error": "invalid_auth"})
		return
	}

	writeJSON(w, map[string]any{
		"ok": true,
		"members": []map[string]any{
			{
				"id": "U01TEST", "name": "testuser", "real_name": "Test User",
				"profile": map[string]string{
					"display_name": "tester", "email": "test@example.com",
					"image_72": "https://example.com/avatar.png",
				},
				"is_bot": false, "is_admin": true, "is_owner": false,
				"deleted": false, "tz": "America/New_York",
			},
			{
				"id": "U02BOT", "name": "mybot", "real_name": "My Bot",
				"profile": map[string]string{
					"display_name": "bot", "email": "",
					"image_72": "",
				},
				"is_bot": true, "is_admin": false, "is_owner": false,
				"deleted": false, "tz": "",
			},
		},
		"response_metadata": map[string]string{"next_cursor": ""},
	})
}

func handleFilesList(w http.ResponseWriter, r *http.Request) {
	if !checkToken(r) {
		writeJSON(w, map[string]any{"ok": false, "error": "invalid_auth"})
		return
	}

	writeJSON(w, map[string]any{
		"ok": true,
		"files": []map[string]any{
			{
				"id": "F01FILE", "name": "report.pdf", "title": "Monthly Report",
				"mimetype": "application/pdf", "size": 102400,
				"url_private": "https://files.slack.com/files/report.pdf",
				"permalink":   "https://testteam.slack.com/files/report.pdf",
				"user": "U01TEST", "created": 1600000010,
			},
		},
		"paging": map[string]int{"page": 1, "pages": 1, "total": 1},
	})
}

func handleSearchMessages(w http.ResponseWriter, r *http.Request) {
	if !checkToken(r) {
		writeJSON(w, map[string]any{"ok": false, "error": "invalid_auth"})
		return
	}

	writeJSON(w, map[string]any{
		"ok": true,
		"messages": map[string]any{
			"total": 1,
			"matches": []map[string]any{
				{
					"channel":   map[string]string{"id": "C01GENERAL", "name": "general"},
					"user":      "U01TEST",
					"text":      "Found this matching message",
					"ts":        "1600000005.000000",
					"permalink": "https://testteam.slack.com/archives/C01GENERAL/p1600000005000000",
				},
			},
			"paging": map[string]int{"page": 1, "pages": 1, "total": 1},
		},
	})
}
