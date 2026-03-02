package scout

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// BridgeMessage represents a message exchanged over the bridge WebSocket.
type BridgeMessage struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"` // "request", "response", "event"
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// BridgeWSHandler processes an incoming bridge WebSocket request and returns a result.
type BridgeWSHandler func(msg BridgeMessage) (any, error)

// bridgeConn represents a single WebSocket client connection.
type bridgeConn struct {
	conn      *websocket.Conn
	pageID    string
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

// BridgeServer provides a WebSocket server for bridge communication between
// Go and browser extensions.
type BridgeServer struct {
	addr     string
	mu       sync.RWMutex
	clients  map[string]*bridgeConn
	handlers map[string]BridgeWSHandler
	listener net.Listener
	server   *http.Server

	// pending tracks in-flight requests waiting for responses.
	pending   map[string]chan *BridgeMessage
	pendingMu sync.Mutex

	// events is the channel for broadcasting bridge events.
	events    chan BridgeEvent
	eventSubs []eventSub
	subMu     sync.RWMutex

	idCounter uint64
	idMu      sync.Mutex
}

type eventSub struct {
	eventType string
	fn        func(BridgeEvent)
}

// NewBridgeServer creates a new WebSocket bridge server bound to the given address.
// The address should be in "host:port" form, e.g. "127.0.0.1:0" for auto-assigned port.
func NewBridgeServer(addr string) *BridgeServer {
	return &BridgeServer{
		addr:     addr,
		clients:  make(map[string]*bridgeConn),
		handlers: make(map[string]BridgeWSHandler),
		pending:  make(map[string]chan *BridgeMessage),
		events:   make(chan BridgeEvent, 256),
	}
}

// Start begins listening for WebSocket connections. It returns once the listener
// is active; connections are handled in background goroutines.
func (s *BridgeServer) Start() error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("scout: bridge: listen: %w", err)
	}

	s.listener = ln

	mux := http.NewServeMux()
	mux.Handle("/bridge", websocket.Handler(s.handleWS))

	s.server = &http.Server{Handler: mux}

	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			// Server stopped unexpectedly — nothing we can do here.
			_ = err
		}
	}()

	return nil
}

// Addr returns the listener address, useful when started on port 0.
func (s *BridgeServer) Addr() string {
	if s == nil || s.listener == nil {
		return ""
	}

	return s.listener.Addr().String()
}

// Stop gracefully shuts down the WebSocket server and closes all client connections.
func (s *BridgeServer) Stop() error {
	if s == nil {
		return nil
	}

	// Close all client connections.
	s.mu.Lock()
	for id, c := range s.clients {
		c.closeOnce.Do(func() { close(c.done) })
		_ = c.conn.Close()

		delete(s.clients, id)
	}
	s.mu.Unlock()

	// Shut down the HTTP server.
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return s.server.Shutdown(ctx)
	}

	return nil
}

// OnMessage registers a handler for a specific method name. When a request with
// the given method arrives from a browser client, the handler is invoked and its
// result is sent back as a response.
func (s *BridgeServer) OnMessage(method string, handler BridgeWSHandler) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers[method] = handler
}

// Send sends a request to a specific page and waits for the response.
// It returns an error if no client with the given pageID is connected or
// if the response is not received within 10 seconds.
func (s *BridgeServer) Send(pageID, method string, params any) (*BridgeMessage, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: bridge: server is nil")
	}

	s.mu.RLock()
	client, ok := s.clients[pageID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("scout: bridge: client %q not connected", pageID)
	}

	id := s.nextID()

	paramsRaw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: marshal params: %w", err)
	}

	msg := BridgeMessage{
		ID:     id,
		Type:   "request",
		Method: method,
		Params: paramsRaw,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: marshal message: %w", err)
	}

	// Register pending response channel.
	ch := make(chan *BridgeMessage, 1)

	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()

	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
	}()

	// Send the message.
	select {
	case client.send <- data:
	default:
		return nil, fmt.Errorf("scout: bridge: send buffer full for %q", pageID)
	}

	// Wait for response with timeout.
	select {
	case resp := <-ch:
		if resp.Error != "" {
			return resp, fmt.Errorf("scout: bridge: remote error: %s", resp.Error)
		}

		return resp, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("scout: bridge: send to %q: timeout after 10s", pageID)
	}
}

// Broadcast sends a message to all connected clients. It does not wait for responses.
func (s *BridgeServer) Broadcast(method string, params any) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	paramsRaw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("scout: bridge: marshal params: %w", err)
	}

	msg := BridgeMessage{
		ID:     s.nextID(),
		Type:   "request",
		Method: method,
		Params: paramsRaw,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("scout: bridge: marshal message: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.clients {
		select {
		case c.send <- data:
		default:
			// Skip clients with full buffers.
		}
	}

	return nil
}

// Clients returns the page IDs of all currently connected clients.
func (s *BridgeServer) Clients() []string {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.clients))
	for id := range s.clients {
		ids = append(ids, id)
	}

	return ids
}

// handleWS is the WebSocket connection handler.
func (s *BridgeServer) handleWS(conn *websocket.Conn) {
	// Read the first message to get the page ID (registration).
	var reg BridgeMessage
	if err := websocket.JSON.Receive(conn, &reg); err != nil {
		_ = conn.Close()
		return
	}

	pageID := reg.Method
	if pageID == "" {
		// Use a fallback identifier from params.
		var p struct {
			PageID string `json:"pageID"`
		}

		_ = json.Unmarshal(reg.Params, &p)
		pageID = p.PageID
	}

	if pageID == "" {
		pageID = conn.Request().RemoteAddr
	}

	c := &bridgeConn{
		conn:   conn,
		pageID: pageID,
		send:   make(chan []byte, 64),
		done:   make(chan struct{}),
	}

	s.mu.Lock()
	s.clients[pageID] = c
	s.mu.Unlock()

	defer func() {
		c.closeOnce.Do(func() { close(c.done) })
		s.mu.Lock()
		delete(s.clients, pageID)
		s.mu.Unlock()

		_ = conn.Close()
	}()

	// Writer goroutine.
	go func() {
		for {
			select {
			case data := <-c.send:
				if _, err := conn.Write(data); err != nil {
					return
				}
			case <-c.done:
				return
			}
		}
	}()

	// Reader loop.
	for {
		var msg BridgeMessage
		if err := websocket.JSON.Receive(conn, &msg); err != nil {
			if err == io.EOF {
				return
			}

			return
		}

		switch msg.Type {
		case "response":
			// Route response to pending request.
			s.pendingMu.Lock()
			ch, ok := s.pending[msg.ID]
			s.pendingMu.Unlock()

			if ok {
				ch <- &msg
			}

		case "event":
			// Emit as a BridgeEvent.
			data := make(map[string]any)
			if msg.Params != nil {
				_ = json.Unmarshal(msg.Params, &data)
			}

			evt := BridgeEvent{
				Type:      msg.Method,
				PageID:    pageID,
				Timestamp: time.Now().UnixMilli(),
				Data:      data,
			}
			// Non-blocking send to events channel.
			select {
			case s.events <- evt:
			default:
			}
			// Notify subscribers.
			s.subMu.RLock()

			for _, sub := range s.eventSubs {
				if sub.eventType == "" || sub.eventType == msg.Method {
					sub.fn(evt)
				}
			}

			s.subMu.RUnlock()

		case "request":
			// Handle incoming request from browser.
			s.mu.RLock()
			handler, ok := s.handlers[msg.Method]
			s.mu.RUnlock()

			if ok {
				go func(m BridgeMessage) {
					result, err := handler(m)

					resp := BridgeMessage{
						ID:   m.ID,
						Type: "response",
					}
					if err != nil {
						resp.Error = err.Error()
					} else {
						resp.Result, _ = json.Marshal(result) //nolint:errchkjson
					}

					data, _ := json.Marshal(resp) //nolint:errchkjson
					select {
					case c.send <- data:
					default:
					}
				}(msg)
			}
		}
	}
}

func (s *BridgeServer) nextID() string {
	s.idMu.Lock()
	defer s.idMu.Unlock()

	s.idCounter++

	return fmt.Sprintf("bs_%d", s.idCounter)
}
