package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Client manages a plugin subprocess and communicates via JSON-RPC 2.0 on stdin/stdout.
type Client struct {
	manifest *Manifest
	cmd      *exec.Cmd
	encoder  *json.Encoder
	scanner  *bufio.Scanner
	logger   *slog.Logger

	mu      sync.Mutex
	pending map[int64]chan *Response
	notify  chan *Notification
	done    chan struct{}
	started bool
}

// NewClient creates a plugin client from a manifest. The subprocess is not started until Start is called.
func NewClient(manifest *Manifest, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		manifest: manifest,
		logger:   logger,
		pending:  make(map[int64]chan *Response),
		notify:   make(chan *Notification, 64),
		done:     make(chan struct{}),
	}
}

// defaultPluginEnvAllowlist is the set of environment variable names every
// plugin subprocess may inherit. It is deliberately a fail-CLOSED allowlist:
// anything not named here (and not opted into via the manifest Env field) is
// dropped, so ambient secrets — vault passphrase, agent API key, pairing
// token, cloud OAuth secret, and any third-party credential (AWS_ACCESS_KEY_ID,
// KUBECONFIG, ...) — never reach an untrusted plugin. It carries only what a
// process needs to start and locate resources, plus the Scout vars plugins
// legitimately read (SCOUT_HOME, SCOUT_PLUGIN_PATH, SCOUT_CDP_ENDPOINT).
var defaultPluginEnvAllowlist = []string{
	// POSIX process basics.
	"PATH", "HOME", "USER", "LOGNAME", "USERNAME", "SHELL",
	"LANG", "LC_ALL", "LC_CTYPE", "TZ",
	"TMPDIR", "TMP", "TEMP",
	// Windows process basics (required for child processes to start / load DLLs).
	"SYSTEMROOT", "SYSTEMDRIVE", "WINDIR", "COMSPEC", "PATHEXT",
	"PROGRAMDATA", "PROGRAMFILES", "PROGRAMFILES(X86)",
	"LOCALAPPDATA", "APPDATA", "USERPROFILE", "HOMEDRIVE", "HOMEPATH",
	"NUMBER_OF_PROCESSORS", "PROCESSOR_ARCHITECTURE",
	// Scout-specific, non-secret vars plugins are expected to read.
	"SCOUT_HOME", "SCOUT_PLUGIN_PATH", "SCOUT_CDP_ENDPOINT",
}

// pluginEnv builds the environment for an untrusted plugin subprocess from a
// fail-closed allowlist (defaultPluginEnvAllowlist) unioned with the names the
// manifest explicitly opts into (Manifest.Env). Matching is case-insensitive to
// honor Windows env semantics. Original "KEY=VALUE" entries are preserved.
func pluginEnv(m *Manifest) []string {
	allowed := make(map[string]bool, len(defaultPluginEnvAllowlist)+len(m.Env))
	for _, name := range defaultPluginEnvAllowlist {
		allowed[strings.ToUpper(name)] = true
	}

	if m != nil {
		for _, name := range m.Env {
			name = strings.TrimSpace(name)
			if name != "" {
				allowed[strings.ToUpper(name)] = true
			}
		}
	}

	parent := os.Environ()
	out := make([]string, 0, len(allowed))
	for _, kv := range parent {
		key, _, ok := strings.Cut(kv, "=")
		if ok && allowed[strings.ToUpper(key)] {
			out = append(out, kv)
		}
	}

	return out
}

// Start launches the plugin subprocess.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return nil
	}

	cmd := exec.CommandContext(ctx, c.manifest.CommandPath())
	cmd.Dir = c.manifest.Dir
	// Untrusted plugin subprocesses receive only a fail-closed allowlist of
	// environment variables (plus manifest-declared opt-ins) — never the full
	// ambient environment — so secrets like the vault passphrase or agent API
	// key cannot leak in. See pluginEnv / defaultPluginEnvAllowlist.
	cmd.Env = pluginEnv(c.manifest)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("plugin: stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("plugin: stdout pipe: %w", err)
	}

	// Stderr is inherited for plugin debug logging.

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("plugin: start %s: %w", c.manifest.Name, err)
	}

	c.cmd = cmd
	c.encoder = json.NewEncoder(stdin)
	c.scanner = bufio.NewScanner(stdout)
	c.scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB line buffer
	c.started = true

	go c.readLoop()

	return nil
}

// readLoop reads JSON-RPC messages from the plugin's stdout.
func (c *Client) readLoop() {
	defer close(c.done)

	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg message
		if err := json.Unmarshal(line, &msg); err != nil {
			c.logger.Warn("plugin: invalid JSON from plugin", "plugin", c.manifest.Name, "error", err)
			continue
		}

		if msg.IsResponse() {
			c.mu.Lock()

			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.mu.Unlock()

			if ok {
				ch <- &Response{
					JSONRPC: msg.JSONRPC,
					ID:      *msg.ID,
					Result:  msg.Result,
					Error:   msg.Error,
				}
			}
		} else if msg.IsNotification() {
			select {
			case c.notify <- &Notification{
				JSONRPC: msg.JSONRPC,
				Method:  msg.Method,
				Params:  msg.Params,
			}:
			default:
				c.logger.Warn("plugin: notification buffer full, dropping", "plugin", c.manifest.Name, "method", msg.Method)
			}
		}
	}

	if err := c.scanner.Err(); err != nil {
		c.logger.Warn("plugin: read error", "plugin", c.manifest.Name, "error", err)
	}
}

// Call sends a JSON-RPC request and waits for the response.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	req, err := NewRequest(method, params)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Response, 1)

	c.mu.Lock()
	c.pending[req.ID] = ch
	c.mu.Unlock()

	if err := c.encoder.Encode(req); err != nil {
		c.mu.Lock()
		delete(c.pending, req.ID)
		c.mu.Unlock()

		return nil, fmt.Errorf("plugin: send request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, req.ID)
		c.mu.Unlock()

		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("plugin: %s process exited", c.manifest.Name)
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}

		return resp.Result, nil
	}
}

// Notify sends a JSON-RPC notification (no response expected) to the plugin.
func (c *Client) Notify(method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}

	return c.encoder.Encode(msg)
}

// Notifications returns the channel for receiving plugin notifications.
func (c *Client) Notifications() <-chan *Notification {
	return c.notify
}

// Initialize performs the handshake with the plugin.
func (c *Client) Initialize(ctx context.Context) error {
	params := map[string]any{
		"capabilities": c.manifest.Capabilities,
	}

	_, err := c.Call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("plugin: initialize %s: %w", c.manifest.Name, err)
	}

	return nil
}

// Shutdown sends a shutdown request and waits for the process to exit.
func (c *Client) Shutdown(ctx context.Context) error {
	if !c.started {
		return nil
	}

	_, _ = c.Call(ctx, "shutdown", nil)

	return c.cmd.Wait()
}

// Done returns a channel that is closed when the plugin process exits.
func (c *Client) Done() <-chan struct{} {
	return c.done
}
