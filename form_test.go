package scout

import (
	"fmt"
	"net/http"
	"testing"
)

func init() {
	registerTestRoutes(formTestRoutes)
}

func formTestRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/form", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Form Test</title></head>
<body>
<form id="login" action="/submit" method="POST">
  <input type="text" name="username" id="username" placeholder="Username" required/>
  <input type="password" name="password" id="password" placeholder="Password"/>
  <select name="role" id="role">
    <option value="user">User</option>
    <option value="admin">Admin</option>
  </select>
  <textarea name="bio" id="bio">default bio</textarea>
  <button type="submit">Login</button>
</form>
<form id="search" action="/search" method="GET">
  <input type="text" name="q" id="q"/>
  <input type="submit" value="Search"/>
</form>
</body></html>`)
	})

	mux.HandleFunc("/form-csrf", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>CSRF Form</title></head>
<body>
<form id="csrf-form" action="/submit" method="POST">
  <input type="hidden" name="csrf_token" value="abc123secret"/>
  <input type="text" name="email"/>
  <button type="submit">Submit</button>
</form>
</body></html>`)
	})

	mux.HandleFunc("/wizard-step1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Wizard Step 1</title></head>
<body>
<div id="step1">
<form id="step-form">
  <input type="text" name="name" id="name"/>
  <button type="button" id="next" onclick="
    document.getElementById('step1').style.display='none';
    document.getElementById('step2').style.display='block';
  ">Next</button>
</form>
</div>
<div id="step2" style="display:none">
<form id="step-form-2">
  <input type="text" name="email" id="email"/>
  <button type="button" id="done" onclick="
    document.getElementById('result').textContent = 'Done: ' + document.getElementById('name').value + ' ' + document.getElementById('email').value;
  ">Finish</button>
</form>
</div>
<div id="result"></div>
</body></html>`)
	})

	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_ = r.ParseForm()
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Submitted</title></head>
<body><div id="result">username=%s</div></body></html>`, r.FormValue("username"))
	})
}

func TestDetectForms(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/form")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	forms, err := page.DetectForms()
	if err != nil {
		t.Fatalf("DetectForms() error: %v", err)
	}
	if len(forms) != 2 {
		t.Fatalf("DetectForms() returned %d forms, want 2", len(forms))
	}

	// Login form
	f := forms[0]
	if f.Action != "/submit" {
		t.Errorf("Action = %q, want /submit", f.Action)
	}
	if f.Method != "POST" {
		t.Errorf("Method = %q, want POST", f.Method)
	}
	if len(f.Fields) < 4 {
		t.Errorf("Fields count = %d, want >= 4", len(f.Fields))
	}
}

func TestDetectForm(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/form")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	f, err := page.DetectForm("#login")
	if err != nil {
		t.Fatalf("DetectForm() error: %v", err)
	}
	if f.Method != "POST" {
		t.Errorf("Method = %q, want POST", f.Method)
	}

	// Check field details
	var usernameField *FormField
	var roleField *FormField
	for i := range f.Fields {
		if f.Fields[i].Name == "username" {
			usernameField = &f.Fields[i]
		}
		if f.Fields[i].Name == "role" {
			roleField = &f.Fields[i]
		}
	}

	if usernameField == nil {
		t.Fatal("username field not found")
	}
	if usernameField.Type != "text" {
		t.Errorf("username type = %q, want text", usernameField.Type)
	}
	if !usernameField.Required {
		t.Error("username should be required")
	}
	if usernameField.Placeholder != "Username" {
		t.Errorf("username placeholder = %q", usernameField.Placeholder)
	}

	if roleField == nil {
		t.Fatal("role field not found")
	}
	if roleField.Type != "select" {
		t.Errorf("role type = %q, want select", roleField.Type)
	}
	if len(roleField.Options) != 2 {
		t.Errorf("role options = %v, want 2 options", roleField.Options)
	}
}

func TestFormFill(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/form")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	f, err := page.DetectForm("#login")
	if err != nil {
		t.Fatalf("DetectForm() error: %v", err)
	}

	if err := f.Fill(map[string]string{
		"username": "testuser",
		"password": "secret123",
		"role":     "admin",
	}); err != nil {
		t.Fatalf("Fill() error: %v", err)
	}

	// Verify the input was set
	result, err := page.Eval(`() => document.getElementById('username').value`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}
	if result.String() != "testuser" {
		t.Errorf("username value = %q, want testuser", result.String())
	}

	// Verify select was set
	result, err = page.Eval(`() => document.getElementById('role').value`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}
	if result.String() != "admin" {
		t.Errorf("role value = %q, want admin", result.String())
	}
}

func TestFormFillStruct(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/form")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	f, err := page.DetectForm("#login")
	if err != nil {
		t.Fatalf("DetectForm() error: %v", err)
	}

	type LoginData struct {
		User string `form:"username"`
		Pass string `form:"password"`
	}

	data := LoginData{User: "structuser", Pass: "structpass"}
	if err := f.FillStruct(data); err != nil {
		t.Fatalf("FillStruct() error: %v", err)
	}

	result, err := page.Eval(`() => document.getElementById('username').value`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}
	if result.String() != "structuser" {
		t.Errorf("username value = %q, want structuser", result.String())
	}
}

func TestFormSubmit(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/form")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	f, err := page.DetectForm("#login")
	if err != nil {
		t.Fatalf("DetectForm() error: %v", err)
	}

	if err := f.Fill(map[string]string{
		"username": "submitted_user",
	}); err != nil {
		t.Fatalf("Fill() error: %v", err)
	}

	wait := page.WaitNavigation()
	if err := f.Submit(); err != nil {
		t.Fatalf("Submit() error: %v", err)
	}
	wait()

	// Check the result page
	text, err := page.ExtractText("#result")
	if err != nil {
		t.Fatalf("ExtractText() error: %v", err)
	}
	if text != "username=submitted_user" {
		t.Errorf("result = %q, want username=submitted_user", text)
	}
}

func TestFormCSRFToken(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/form-csrf")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	f, err := page.DetectForm("#csrf-form")
	if err != nil {
		t.Fatalf("DetectForm() error: %v", err)
	}

	token, err := f.CSRFToken()
	if err != nil {
		t.Fatalf("CSRFToken() error: %v", err)
	}
	if token != "abc123secret" {
		t.Errorf("CSRFToken() = %q, want abc123secret", token)
	}
}

func TestFormCSRFTokenNotFound(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/form")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	f, err := page.DetectForm("#login")
	if err != nil {
		t.Fatalf("DetectForm() error: %v", err)
	}

	_, err = f.CSRFToken()
	if err == nil {
		t.Error("CSRFToken() should return error when no token found")
	}
}

func TestFormWizard(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/wizard-step1")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	wizard := page.NewFormWizard(
		WizardStep{
			FormSelector: "#step-form",
			Data:         map[string]string{"name": "John"},
			NextSelector: "#next",
			WaitFor:      "#step1",
		},
		WizardStep{
			FormSelector: "#step-form-2",
			Data:         map[string]string{"email": "john@test.com"},
			NextSelector: "#done",
			WaitFor:      "#step2",
		},
	)

	if err := wizard.Run(); err != nil {
		t.Fatalf("Wizard.Run() error: %v", err)
	}

	result, err := page.ExtractText("#result")
	if err != nil {
		t.Fatalf("ExtractText() error: %v", err)
	}
	if result != "Done: John john@test.com" {
		t.Errorf("result = %q, want 'Done: John john@test.com'", result)
	}
}
