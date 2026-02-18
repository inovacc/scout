package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// testEnv holds a running gRPC server, httptest fixture server, and client.
type testEnv struct {
	client  pb.ScoutServiceClient
	baseURL string // httptest server URL
}

// setupTestServer starts an httptest server with fixture routes and an in-process
// insecure gRPC ScoutServer. Returns client + base URL. Cleans up on test end.
func setupTestServer(t *testing.T) *testEnv {
	t.Helper()

	// HTTP fixture server
	mux := http.NewServeMux()
	registerFixtureRoutes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// gRPC server
	srv := New()
	grpcServer := grpc.NewServer()
	pb.RegisterScoutServiceServer(grpcServer, srv)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(grpcServer.Stop)

	// gRPC client
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return &testEnv{
		client:  pb.NewScoutServiceClient(conn),
		baseURL: ts.URL,
	}
}

// createSession is a helper that creates a headless session and skips the test
// if the browser is unavailable.
func (e *testEnv) createSession(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := e.client.CreateSession(ctx, &pb.CreateSessionRequest{
		Headless:  true,
		NoSandbox: true,
	})
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}

	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		_, _ = e.client.DestroySession(ctx2, &pb.SessionRequest{SessionId: resp.SessionId})
	})

	return resp.SessionId
}

// navigate is a shorthand to navigate and wait stable.
func (e *testEnv) navigate(t *testing.T, sessionID, path string) *pb.NavigateResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := e.client.Navigate(ctx, &pb.NavigateRequest{
		SessionId:  sessionID,
		Url:        e.baseURL + path,
		WaitStable: true,
	})
	if err != nil {
		t.Fatalf("navigate %s: %v", path, err)
	}

	return resp
}

// registerFixtureRoutes adds minimal HTML fixture routes to the mux.
func registerFixtureRoutes(mux *http.ServeMux) {
	// Main test page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Test Page</title></head>
<body>
<h1>Hello World</h1>
<p id="info">Some text</p>
<input id="name" type="text" value="default"/>
<button id="btn" onclick="document.getElementById('info').textContent='Clicked'">Click Me</button>
<a id="link" href="/page2">Go to page 2</a>
<select id="sel">
  <option value="a">Alpha</option>
  <option value="b">Beta</option>
  <option value="c">Gamma</option>
</select>
<div id="parent"><span id="child">Child Text</span></div>
<div id="hidden" style="display:none">Hidden</div>
</body></html>`)
	})

	// Second page
	mux.HandleFunc("/page2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Page Two</title></head>
<body><h1>Page 2</h1><a href="/">Back</a></body></html>`)
	})

	// Form page
	mux.HandleFunc("/form", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Form Page</title></head>
<body>
<form id="myform" action="/submit" method="post">
  <input id="fname" name="fname" type="text" />
  <input id="lname" name="lname" type="text" />
  <select id="color" name="color">
    <option value="red">Red</option>
    <option value="blue">Blue</option>
    <option value="green">Green</option>
  </select>
  <button id="submit-btn" type="submit">Submit</button>
</form>
</body></html>`)
	})

	// Form submit endpoint
	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Submitted</title></head>
<body><p id="result">fname=%s lname=%s</p></body></html>`,
			r.FormValue("fname"), r.FormValue("lname"))
	})

	// Click target page
	mux.HandleFunc("/click-target", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Click Target</title></head>
<body>
<button id="btn" onclick="document.getElementById('output').textContent='clicked'">Click</button>
<button id="dbl" ondblclick="document.getElementById('output').textContent='double-clicked'">DblClick</button>
<div id="hover-zone" onmouseenter="document.getElementById('output').textContent='hovered'">Hover Here</div>
<p id="output">ready</p>
</body></html>`)
	})

	// Recorder asset
	mux.HandleFunc("/recorder-asset", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	// Page that fetches an asset (for HAR recording)
	mux.HandleFunc("/recorder-page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// Use the same host for the fetch so it goes through the test server
		fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Recorder</title></head>
<body>
<script>fetch('%s/recorder-asset')</script>
<p>Recording test</p>
</body></html>`, "http://"+r.Host)
	})
}
