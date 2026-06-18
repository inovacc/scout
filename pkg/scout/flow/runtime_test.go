package flow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRESTExtractInjectChain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Csrf-Token", "csrf-9")
		_, _ = w.Write([]byte(`{"access_token":"tok-9"}`))
	})
	var sawAuth, sawCSRF string
	mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		sawCSRF = r.Header.Get("X-Csrf-Token")
		_, _ = w.Write([]byte(`{"id":"u-1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := &FlowSpec{Version: "1", Name: "t", Vars: map[string]string{"baseURL": srv.URL},
		Steps: []FlowStep{
			{ID: "login", Request: Request{Method: "POST", URL: "${baseURL}/login"},
				Extract: []Extract{
					{Var: "token", From: "response.json", Path: "$.access_token"},
					{Var: "csrf", From: "response.header", Path: "X-CSRF-Token"},
				}, Expect: &Expect{Status: 200}},
			{ID: "me", Request: Request{Method: "GET", URL: "${baseURL}/me",
				Headers: map[string]string{"Authorization": "Bearer ${token}", "X-CSRF-Token": "${csrf}"}},
				Extract: []Extract{{Var: "uid", From: "response.json", Path: "$.id"}}},
		}}
	if err := Validate(f); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), f, RunOptions{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sawAuth != "Bearer tok-9" || sawCSRF != "csrf-9" {
		t.Fatalf("injection failed: auth=%q csrf=%q", sawAuth, sawCSRF)
	}
	if res.Steps[1].Extracted["uid"] != "u-1" {
		t.Fatalf("final extract failed: %+v", res.Steps[1].Extracted)
	}
}

func TestRunExpectStatusMismatchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	f := &FlowSpec{Version: "1", Steps: []FlowStep{
		{ID: "x", Request: Request{Method: "GET", URL: srv.URL}, Expect: &Expect{Status: 200}}}}
	if _, err := Run(context.Background(), f, RunOptions{}); err == nil {
		t.Fatal("expected expect-mismatch error")
	}
}

func TestRunGraphQL(t *testing.T) {
	var gotOp, gotVar string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotOp = body.OperationName
		if v, ok := body.Variables["id"].(string); ok {
			gotVar = v
		}
		_, _ = w.Write([]byte(`{"data":{"cart":{"total":7}}}`))
	}))
	defer srv.Close()

	f := &FlowSpec{Version: "1", Vars: map[string]string{"cartId": "c-7"}, Steps: []FlowStep{
		{ID: "cart", Request: Request{Method: "POST", URL: srv.URL, GraphQL: &GraphQL{
			OperationName: "Cart", Query: "query Cart($id:ID!){cart(id:$id){total}}",
			Variables: map[string]any{"id": "${cartId}"}}},
			Extract: []Extract{{Var: "total", From: "response.json", Path: "$.data.cart.total"}}}}}
	res, err := Run(context.Background(), f, RunOptions{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotOp != "Cart" || gotVar != "c-7" {
		t.Fatalf("graphql body wrong: op=%q id=%q", gotOp, gotVar)
	}
	if res.Steps[0].Extracted["total"] != "7" {
		t.Fatalf("extract: %+v", res.Steps[0].Extracted)
	}
}

func TestRunDumpDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	spec := &FlowSpec{
		Name:  "t",
		Steps: []FlowStep{{ID: "one", Request: Request{Method: "GET", URL: srv.URL}}},
	}
	dir := t.TempDir()
	_, err := Run(context.Background(), spec, RunOptions{DumpDir: dir})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "one.json"))
	if err != nil || string(b) != `{"ok":true}` {
		t.Fatalf("dump = %q err=%v", b, err)
	}
}

// guard for unused imports in some toolchains
var _ = io.Discard
var _ = json.Marshal
