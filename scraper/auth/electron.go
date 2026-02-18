package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/inovacc/scout/pkg/rod"
	"github.com/inovacc/scout/pkg/rod/lib/proto"
	"github.com/inovacc/scout/pkg/scout"
)

// ElectronOptions configures connection to an Electron app's debug port.
type ElectronOptions struct {
	// DebugPort is the Chrome DevTools Protocol port.
	DebugPort int

	// Timeout for connection attempts.
	Timeout time.Duration
}

// ElectronSession connects to a running Electron app via CDP and captures
// session data from the first available page target.
func ElectronSession(ctx context.Context, opts ElectronOptions) (*Session, error) {
	if opts.DebugPort == 0 {
		return nil, fmt.Errorf("auth: electron: debug port is required (launch app with --remote-debugging-port=PORT)")
	}

	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	debugURL := fmt.Sprintf("http://127.0.0.1:%d", opts.DebugPort)

	// Wait for the debug endpoint to be available
	deadline := time.Now().Add(opts.Timeout)

	var wsURL string

	for time.Now().Before(deadline) {
		u, err := getDebuggerWebSocketURL(debugURL)
		if err == nil {
			wsURL = u

			break
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("auth: electron: %w", ctx.Err())
		default:
		}

		time.Sleep(500 * time.Millisecond)
	}

	if wsURL == "" {
		return nil, fmt.Errorf("auth: electron: could not connect to debug port %d within %v", opts.DebugPort, opts.Timeout)
	}

	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("auth: electron: connect: %w", err)
	}

	defer func() { _ = browser.Close() }()

	pages, err := browser.Pages()
	if err != nil {
		return nil, fmt.Errorf("auth: electron: list pages: %w", err)
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("auth: electron: no pages found")
	}

	page := pages.First()

	// Extract cookies
	cookies, err := page.Cookies(nil)
	if err != nil {
		return nil, fmt.Errorf("auth: electron: get cookies: %w", err)
	}

	ls, err := evalStorage(page, "localStorage")
	if err != nil {
		ls = make(map[string]string)
	}

	ss, err := evalStorage(page, "sessionStorage")
	if err != nil {
		ss = make(map[string]string)
	}

	info, err := page.Info()
	if err != nil {
		return nil, fmt.Errorf("auth: electron: page info: %w", err)
	}

	session := &Session{
		Provider:       "electron",
		Version:        "1.0",
		Timestamp:      time.Now().UTC(),
		URL:            info.URL,
		LocalStorage:   ls,
		SessionStorage: ss,
	}

	for _, c := range cookies {
		session.Cookies = append(session.Cookies, protoCookieToScout(c))
	}

	return session, nil
}

func getDebuggerWebSocketURL(debugURL string) (string, error) {
	resp, err := http.Get(debugURL + "/json/version") //nolint:gosec,noctx
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	var result struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("no WebSocket URL in response")
	}

	return result.WebSocketDebuggerURL, nil
}

func evalStorage(page *rod.Page, storageName string) (map[string]string, error) {
	js := fmt.Sprintf(`() => {
		const result = {};
		try {
			for (let i = 0; i < %s.length; i++) {
				const key = %s.key(i);
				result[key] = %s.getItem(key);
			}
		} catch(e) {}
		return JSON.stringify(result);
	}`, storageName, storageName, storageName)

	result, err := page.Eval(js)
	if err != nil {
		return nil, err
	}

	var data map[string]string
	if err := json.Unmarshal([]byte(result.Value.Str()), &data); err != nil {
		return nil, err
	}

	return data, nil
}

func protoCookieToScout(c *proto.NetworkCookie) scout.Cookie {
	return scout.Cookie{
		Name:     c.Name,
		Value:    c.Value,
		Domain:   c.Domain,
		Path:     c.Path,
		Secure:   c.Secure,
		HTTPOnly: c.HTTPOnly,
		SameSite: string(c.SameSite),
	}
}
