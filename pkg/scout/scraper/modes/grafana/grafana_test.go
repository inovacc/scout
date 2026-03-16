package grafana

import (
	"fmt"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestGrafanaMode_Name(t *testing.T) {
	m := &GrafanaMode{}
	if got := m.Name(); got != "grafana" {
		t.Errorf("Name() = %q, want %q", got, "grafana")
	}
}

func TestGrafanaMode_Description(t *testing.T) {
	m := &GrafanaMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestGrafanaMode_AuthProvider(t *testing.T) {
	m := &GrafanaMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "grafana" {
		t.Errorf("AuthProvider().Name() = %q", p.Name())
	}
}

// --- grafanaProvider tests ---

func TestGrafanaProvider_LoginURL(t *testing.T) {
	p := &grafanaProvider{}
	if got := p.LoginURL(); got != "https://grafana.com/auth/sign-in" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &grafanaProvider{}
	if err := p.ValidateSession(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateSession_ValidGrafanaSession(t *testing.T) {
	p := &grafanaProvider{}
	s := &auth.Session{Tokens: map[string]string{"grafana_session": "tok"}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidAPIToken(t *testing.T) {
	p := &grafanaProvider{}
	s := &auth.Session{Tokens: map[string]string{"api_token": "key"}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidCookie(t *testing.T) {
	p := &grafanaProvider{}
	s := &auth.Session{
		Tokens:  map[string]string{},
		Cookies: []scout.Cookie{{Name: "grafana_session", Value: "val"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_NoAuth(t *testing.T) {
	p := &grafanaProvider{}
	s := &auth.Session{
		Tokens:  map[string]string{},
		Cookies: []scout.Cookie{{Name: "other", Value: "val"}},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*scraper.AuthError); !ok {
		t.Errorf("expected *scraper.AuthError, got %T", err)
	}
}

// --- buildTargetSet tests ---

func TestBuildTargetSet_Empty(t *testing.T) {
	if set := buildTargetSet(nil); set != nil {
		t.Errorf("expected nil, got %v", set)
	}
}

func TestBuildTargetSet_Normalized(t *testing.T) {
	set := buildTargetSet([]string{"  DashUID  "})
	if _, ok := set["dashuid"]; !ok {
		t.Error("expected trimmed+lowered key")
	}
}

// --- parseDashboardsList tests ---

func TestParseDashboardsList_Valid(t *testing.T) {
	body := `{"results": [{"uid": "abc", "title": "Main Dashboard", "id": 1, "folder": "General", "folderId": 0, "type": "dash-db", "tags": ["prod"]}]}`
	results := parseDashboardsList(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	r := results[0]
	if r.Type != scraper.ResultPost {
		t.Errorf("Type = %q", r.Type)
	}
	if r.ID != "abc" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Content != "Main Dashboard" {
		t.Errorf("Content = %q", r.Content)
	}
}

func TestParseDashboardsList_WithFilter(t *testing.T) {
	body := `{"results": [{"uid": "abc", "title": "D1"}, {"uid": "def", "title": "D2"}]}`
	targetSet := buildTargetSet([]string{"abc"})
	results := parseDashboardsList(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
}

func TestParseDashboardsList_InvalidJSON(t *testing.T) {
	results := parseDashboardsList("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseDashboardDetail tests ---

func TestParseDashboardDetail_Valid(t *testing.T) {
	body := `{
		"dashboard": {
			"id": 1, "uid": "abc", "title": "My Dash", "tags": ["env:prod"],
			"panels": [
				{"id": 1, "title": "CPU Usage", "type": "graph"},
				{"id": 2, "title": "Memory", "type": "gauge"}
			],
			"timezone": "utc", "schemaVersion": 38
		}
	}`
	results := parseDashboardDetail(body, nil)
	// 1 dashboard + 2 panels = 3 results
	if len(results) != 3 {
		t.Fatalf("got %d, want 3", len(results))
	}
	if results[0].Content != "My Dash" {
		t.Errorf("results[0].Content = %q", results[0].Content)
	}
	if results[0].Metadata["panel_count"] != 2 {
		t.Errorf("panel_count = %v", results[0].Metadata["panel_count"])
	}
	if results[1].Type != scraper.ResultFile {
		t.Errorf("panel Type = %q", results[1].Type)
	}
	if results[1].ID != "abc_panel_1" {
		t.Errorf("panel ID = %q", results[1].ID)
	}
}

func TestParseDashboardDetail_FilteredOut(t *testing.T) {
	body := `{"dashboard": {"uid": "abc", "title": "D1"}}`
	targetSet := buildTargetSet([]string{"other"})
	results := parseDashboardDetail(body, targetSet)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseDatasourcesList tests ---

func TestParseDatasourcesList_Valid(t *testing.T) {
	body := `[{"id": 1, "uid": "ds1", "name": "Prometheus", "type": "prometheus", "url": "http://localhost:9090", "isDefault": true}]`
	results := parseDatasourcesList(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Content != "Prometheus" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestParseDatasourcesList_InvalidJSON(t *testing.T) {
	results := parseDatasourcesList("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseAlertsList tests ---

func TestParseAlertsList_Valid(t *testing.T) {
	body := fmt.Sprintf(`{"results": [{"id": 1, "dashboardId": 10, "dashboardUid": "abc", "name": "CPU Alert", "state": "alerting", "message": "CPU > 90%%", "created": %d, "updated": %d}]}`, int64(1609459200000), int64(1609459300000))
	results := parseAlertsList(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultMessage {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Author != "CPU Alert" {
		t.Errorf("Author = %q", results[0].Author)
	}
}

// --- parseSearchResults tests ---

func TestParseSearchResults_Valid(t *testing.T) {
	body := `{"results": [
		{"id": 1, "uid": "d1", "title": "Dashboard", "type": "dash-db", "url": "/d/d1"},
		{"id": 2, "uid": "f1", "title": "Folder", "type": "folder", "url": "/dashboards/f/f1"}
	]}`
	results := parseSearchResults(body, nil)
	if len(results) != 2 {
		t.Fatalf("got %d, want 2", len(results))
	}
	if results[0].Type != scraper.ResultPost {
		t.Errorf("results[0].Type = %q", results[0].Type)
	}
	if results[1].Type != scraper.ResultChannel {
		t.Errorf("results[1].Type = %q, want channel for folder", results[1].Type)
	}
}

func TestParseSearchResults_WithFilter(t *testing.T) {
	body := `{"results": [{"uid": "d1", "title": "D1", "type": "dash-db"}, {"uid": "d2", "title": "D2", "type": "dash-db"}]}`
	targetSet := buildTargetSet([]string{"d1"})
	results := parseSearchResults(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
}

// --- parsePanelQuery tests ---

func TestParsePanelQuery_Valid(t *testing.T) {
	body := `{"results": [{"status": 200, "meta": {"key": "value"}}]}`
	results := parsePanelQuery(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultFile {
		t.Errorf("Type = %q", results[0].Type)
	}
}

// --- parseAnnotations tests ---

func TestParseAnnotations_Valid(t *testing.T) {
	body := fmt.Sprintf(`{"results": [{"id": 1, "dashboardId": 10, "text": "Deploy v2.0", "time": %d, "tags": ["deploy"]}]}`, int64(1609459200000))
	results := parseAnnotations(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultComment {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Content != "Deploy v2.0" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

// --- parseHijackEvent tests ---

func TestParseHijackEvent_NonResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventRequest}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseHijackEvent_EmptyBody(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://grafana.example.com/api/dashboards/find", Body: ""},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseHijackEvent_UnknownURL(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://example.com/unknown", Body: "{}"},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}
