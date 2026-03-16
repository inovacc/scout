package salesforce

import (
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestSalesforceMode_Name(t *testing.T) {
	m := &SalesforceMode{}
	if got := m.Name(); got != "salesforce" {
		t.Errorf("Name() = %q, want %q", got, "salesforce")
	}
}

func TestSalesforceMode_Description(t *testing.T) {
	m := &SalesforceMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestSalesforceMode_AuthProvider(t *testing.T) {
	m := &SalesforceMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "salesforce" {
		t.Errorf("AuthProvider().Name() = %q", p.Name())
	}
}

// --- salesforceProvider tests ---

func TestSalesforceProvider_LoginURL(t *testing.T) {
	p := &salesforceProvider{}
	if got := p.LoginURL(); got != "https://login.salesforce.com/" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &salesforceProvider{}
	if err := p.ValidateSession(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateSession_ValidSidCookie(t *testing.T) {
	p := &salesforceProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "sid", Value: "abc"}}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidOidCookie(t *testing.T) {
	p := &salesforceProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "oid", Value: "xyz"}}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidAccessToken(t *testing.T) {
	p := &salesforceProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{},
		Tokens:  map[string]string{"access_token": "tok"},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_NoAuth(t *testing.T) {
	p := &salesforceProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "other", Value: "val"}},
		Tokens:  map[string]string{},
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

func TestBuildTargetSet_Uppercase(t *testing.T) {
	set := buildTargetSet([]string{"Lead", "contact"})
	if _, ok := set["LEAD"]; !ok {
		t.Error("expected LEAD")
	}
	if _, ok := set["CONTACT"]; !ok {
		t.Error("expected CONTACT")
	}
}

// --- parseSalesforceTimestamp tests ---

func TestParseSalesforceTimestamp(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		isZero bool
	}{
		{"empty", "", true},
		{"rfc3339", "2024-02-28T10:30:45Z", false},
		{"simple", "2024-02-28T10:30:45", false},
		{"invalid", "not-a-date", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSalesforceTimestamp(tt.input)
			if got.IsZero() != tt.isZero {
				t.Errorf("parseSalesforceTimestamp(%q).IsZero() = %v, want %v", tt.input, got.IsZero(), tt.isZero)
			}
		})
	}
}

func TestParseSalesforceTimestamp_CorrectValue(t *testing.T) {
	got := parseSalesforceTimestamp("2024-02-28T10:30:45Z")
	want := time.Date(2024, 2, 28, 10, 30, 45, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// --- parseLeadsResponse tests ---

func TestParseLeadsResponse_Valid(t *testing.T) {
	body := `{
		"totalSize": 1,
		"done": true,
		"records": [
			{"Id": "L001", "FirstName": "John", "LastName": "Doe", "Company": "Acme", "Email": "john@acme.com", "Phone": "555-1234", "Status": "Open", "CreatedDate": "2024-01-01T00:00:00Z", "Industry": "Tech", "LeadSource": "Web"}
		]
	}`
	results := parseLeadsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	r := results[0]
	if r.Type != scraper.ResultProfile {
		t.Errorf("Type = %q", r.Type)
	}
	if r.Author != "John Doe" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Content != "Acme" {
		t.Errorf("Content = %q", r.Content)
	}
	if r.Metadata["email"] != "john@acme.com" {
		t.Errorf("Metadata[email] = %v", r.Metadata["email"])
	}
}

func TestParseLeadsResponse_FilteredByTargetSet(t *testing.T) {
	body := `{"records": [{"Id": "L001", "FirstName": "A", "LastName": "B"}]}`
	targetSet := buildTargetSet([]string{"Contact"}) // not LEAD
	results := parseLeadsResponse(body, targetSet)
	if len(results) != 0 {
		t.Errorf("expected 0 for filtered target, got %d", len(results))
	}
}

func TestParseLeadsResponse_SkipsEmptyID(t *testing.T) {
	body := `{"records": [{"Id": "", "FirstName": "A"}]}`
	results := parseLeadsResponse(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseLeadsResponse_InvalidJSON(t *testing.T) {
	results := parseLeadsResponse("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseContactsResponse tests ---

func TestParseContactsResponse_Valid(t *testing.T) {
	body := `{"records": [{"Id": "C001", "FirstName": "Jane", "LastName": "Smith", "Email": "jane@test.com", "Title": "CEO", "CreatedDate": "2024-01-01T00:00:00Z"}]}`
	results := parseContactsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Content != "CEO" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestParseContactsResponse_FilteredOut(t *testing.T) {
	body := `{"records": [{"Id": "C001"}]}`
	targetSet := buildTargetSet([]string{"Lead"})
	results := parseContactsResponse(body, targetSet)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseOpportunitiesResponse tests ---

func TestParseOpportunitiesResponse_Valid(t *testing.T) {
	body := `{"records": [{"Id": "O001", "Name": "Big Deal", "StageName": "Proposal", "Amount": 50000, "Description": "Enterprise license", "CreatedDate": "2024-01-01T00:00:00Z"}]}`
	results := parseOpportunitiesResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultPost {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Metadata["amount"] != float64(50000) {
		t.Errorf("Metadata[amount] = %v", results[0].Metadata["amount"])
	}
}

// --- parseAccountsResponse tests ---

func TestParseAccountsResponse_Valid(t *testing.T) {
	body := `{"records": [{"Id": "A001", "Name": "Acme Corp", "Industry": "Manufacturing", "AnnualRevenue": 1000000, "NumberOfEmployees": 500, "CreatedDate": "2024-01-01T00:00:00Z"}]}`
	results := parseAccountsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Content != "Manufacturing" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

// --- parseReportsResponse tests ---

func TestParseReportsResponse_Valid(t *testing.T) {
	body := `{"records": [{"Id": "R001", "Name": "Q4 Report", "Description": "Quarterly", "Owner": "Admin", "CreatedDate": "2024-01-01T00:00:00Z", "ReportType": "Tabular"}]}`
	results := parseReportsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultFile {
		t.Errorf("Type = %q", results[0].Type)
	}
}

// --- parseTasksResponse tests ---

func TestParseTasksResponse_Valid(t *testing.T) {
	body := `{"records": [{"Id": "T001", "Subject": "Follow up", "Status": "Open", "Priority": "High", "Owner": "Alice", "CreatedDate": "2024-01-01T00:00:00Z"}]}`
	results := parseTasksResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultMessage {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Metadata["priority"] != "High" {
		t.Errorf("Metadata[priority] = %v", results[0].Metadata["priority"])
	}
}

func TestParseTasksResponse_FilteredOut(t *testing.T) {
	body := `{"records": [{"Id": "T001", "Subject": "x"}]}`
	targetSet := buildTargetSet([]string{"Lead"})
	results := parseTasksResponse(body, targetSet)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseUIAPIResponse tests ---

func TestParseUIAPIResponse_SingleObject(t *testing.T) {
	body := `{"Id": "UI001", "ApiName": "Lead", "DisplayName": "John"}`
	results := parseUIAPIResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultProfile {
		t.Errorf("Type = %q, want profile for Lead", results[0].Type)
	}
}

func TestParseUIAPIResponse_Array(t *testing.T) {
	body := `[{"Id": "UI001", "ApiName": "Account", "DisplayName": "Acme"}, {"Id": "UI002", "ApiName": "Task", "DisplayName": "Follow up"}]`
	results := parseUIAPIResponse(body, nil)
	if len(results) != 2 {
		t.Fatalf("got %d, want 2", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("results[0].Type = %q", results[0].Type)
	}
	if results[1].Type != scraper.ResultMessage {
		t.Errorf("results[1].Type = %q", results[1].Type)
	}
}

func TestParseUIAPIResponse_SkipsEmptyID(t *testing.T) {
	body := `[{"Id": "", "ApiName": "Lead"}]`
	results := parseUIAPIResponse(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseUIAPIResponse_InvalidJSON(t *testing.T) {
	results := parseUIAPIResponse("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
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
		Response: &scout.CapturedResponse{URL: "https://instance.salesforce.com/services/data/v58/sobjects/Lead", Body: ""},
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
