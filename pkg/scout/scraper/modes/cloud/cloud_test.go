package cloud

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestCloudMode_Name(t *testing.T) {
	m := &CloudMode{}
	if got := m.Name(); got != "cloud" {
		t.Errorf("Name() = %q, want %q", got, "cloud")
	}
}

func TestCloudMode_Description(t *testing.T) {
	m := &CloudMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestCloudMode_AuthProvider(t *testing.T) {
	m := &CloudMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "cloud" {
		t.Errorf("AuthProvider().Name() = %q", p.Name())
	}
}

// --- cloudProvider tests ---

func TestCloudProvider_LoginURL(t *testing.T) {
	p := &cloudProvider{}
	if got := p.LoginURL(); got != "https://console.aws.amazon.com/" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &cloudProvider{}
	if err := p.ValidateSession(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateSession_ValidAWSToken(t *testing.T) {
	p := &cloudProvider{}
	s := &auth.Session{Tokens: map[string]string{"aws-userinfo": "data"}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidGCPToken(t *testing.T) {
	p := &cloudProvider{}
	s := &auth.Session{Tokens: map[string]string{"gcp-osid": "osid"}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidAzureToken(t *testing.T) {
	p := &cloudProvider{}
	s := &auth.Session{Tokens: map[string]string{"azure-idtoken": "tok"}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidAWSLocalStorage(t *testing.T) {
	p := &cloudProvider{}
	s := &auth.Session{
		Tokens:       map[string]string{},
		LocalStorage: map[string]string{"aws-userInfo": "data"},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidAzureSessionStorage(t *testing.T) {
	p := &cloudProvider{}
	s := &auth.Session{
		Tokens:         map[string]string{},
		SessionStorage: map[string]string{"azure-subscription": "sub"},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_NoAuth(t *testing.T) {
	p := &cloudProvider{}
	s := &auth.Session{
		Tokens:         map[string]string{},
		LocalStorage:   map[string]string{},
		SessionStorage: map[string]string{},
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
	set := buildTargetSet([]string{"  EC2  ", "s3"})
	if _, ok := set["ec2"]; !ok {
		t.Error("expected trimmed+lowered 'ec2'")
	}
	if _, ok := set["s3"]; !ok {
		t.Error("expected 's3'")
	}
}

// --- parseAWSEC2 tests ---

func TestParseAWSEC2_Valid(t *testing.T) {
	body := `{
		"Reservations": [
			{
				"Instances": [
					{"InstanceId": "i-1234", "State": {"Name": "running"}}
				]
			}
		]
	}`
	results := parseAWSEC2(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].ID != "i-1234" {
		t.Errorf("ID = %q", results[0].ID)
	}
	if results[0].Source != "aws" {
		t.Errorf("Source = %q", results[0].Source)
	}
	if results[0].Content != "running" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestParseAWSEC2_EmptyInstanceID(t *testing.T) {
	body := `{"Reservations": [{"Instances": [{"InstanceId": ""}]}]}`
	results := parseAWSEC2(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseAWSEC2_InvalidJSON(t *testing.T) {
	results := parseAWSEC2("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseAWSEC2_WithTargetFilter(t *testing.T) {
	body := `{"Reservations": [{"Instances": [{"InstanceId": "i-1234", "State": {"Name": "running"}}]}]}`
	targetSet := buildTargetSet([]string{"i-1234"})
	results := parseAWSEC2(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
}

func TestParseAWSEC2_FilteredByService(t *testing.T) {
	body := `{"Reservations": [{"Instances": [{"InstanceId": "i-1234", "State": {"Name": "running"}}]}]}`
	targetSet := buildTargetSet([]string{"ec2"})
	results := parseAWSEC2(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1 (ec2 service filter)", len(results))
	}
}

func TestParseAWSEC2_FilteredOut(t *testing.T) {
	body := `{"Reservations": [{"Instances": [{"InstanceId": "i-1234", "State": {"Name": "running"}}]}]}`
	targetSet := buildTargetSet([]string{"s3"})
	results := parseAWSEC2(body, targetSet)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseAWSS3 tests ---

func TestParseAWSS3_Valid(t *testing.T) {
	body := `{"Buckets": [{"Name": "my-bucket", "CreationDate": "2024-01-01"}]}`
	results := parseAWSS3(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].ID != "my-bucket" {
		t.Errorf("ID = %q", results[0].ID)
	}
}

func TestParseAWSS3_EmptyBucketName(t *testing.T) {
	body := `{"Buckets": [{"Name": ""}]}`
	results := parseAWSS3(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseAWSIAM tests ---

func TestParseAWSIAM_Users(t *testing.T) {
	body := `{"Users": [{"UserName": "admin", "Arn": "arn:aws:iam::123:user/admin"}]}`
	results := parseAWSIAM(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultUser {
		t.Errorf("Type = %q", results[0].Type)
	}
}

func TestParseAWSIAM_Roles(t *testing.T) {
	body := `{"Roles": [{"RoleName": "dev-role", "Arn": "arn:aws:iam::123:role/dev-role"}]}`
	results := parseAWSIAM(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q", results[0].Type)
	}
}

// --- parseAWSPricing tests ---

func TestParseAWSPricing_Valid(t *testing.T) {
	body := `{"products": [{"sku": "abc"}]}`
	results := parseAWSPricing(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultMessage {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].ID != "pricing-snapshot" {
		t.Errorf("ID = %q", results[0].ID)
	}
}

// --- parseGCPResources tests ---

func TestParseGCPResources_Valid(t *testing.T) {
	body := `{"resources": [{"name": "my-vm"}]}`
	results := parseGCPResources(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Source != "gcp" {
		t.Errorf("Source = %q", results[0].Source)
	}
}

func TestParseGCPResources_EmptyName(t *testing.T) {
	body := `{"resources": [{"name": ""}]}`
	results := parseGCPResources(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseGCPProjects tests ---

func TestParseGCPProjects_Valid(t *testing.T) {
	body := `{"projects": [{"projectId": "my-project", "name": "My Project", "projectNumber": "123", "lifecycleState": "ACTIVE"}]}`
	results := parseGCPProjects(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Content != "My Project" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

// --- parseAzureResources tests ---

func TestParseAzureResources_Valid(t *testing.T) {
	body := `{"value": [{"id": "/subscriptions/sub1/resourceGroups/rg1", "name": "my-vm", "type": "Microsoft.Compute/virtualMachines", "location": "eastus"}]}`
	results := parseAzureResources(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Source != "azure" {
		t.Errorf("Source = %q", results[0].Source)
	}
}

func TestParseAzureResources_EmptyNameOrID(t *testing.T) {
	body := `{"value": [{"id": "", "name": ""}, {"id": "x", "name": ""}]}`
	results := parseAzureResources(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseAzureAPI tests ---

func TestParseAzureAPI_Subscriptions(t *testing.T) {
	body := `{"data": {"subscriptions": [{"subscriptionId": "sub-1", "displayName": "Dev Sub", "state": "Enabled"}]}}`
	results := parseAzureAPI(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q", results[0].Type)
	}
}

func TestParseAzureAPI_ValueFallback(t *testing.T) {
	body := `{"value": [{"id": "item-1", "name": "Resource1"}]}`
	results := parseAzureAPI(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultPost {
		t.Errorf("Type = %q", results[0].Type)
	}
}

func TestParseAzureAPI_InvalidJSON(t *testing.T) {
	results := parseAzureAPI("bad", nil)
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
		Response: &scout.CapturedResponse{URL: "https://console.aws.amazon.com/ec2", Body: ""},
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

func TestParseHijackEvent_EC2Route(t *testing.T) {
	body := `{"Reservations": [{"Instances": [{"InstanceId": "i-test", "State": {"Name": "running"}}]}]}`
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://console.aws.amazon.com/ec2/api", Body: body},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
}
