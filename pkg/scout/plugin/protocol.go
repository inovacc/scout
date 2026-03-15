package plugin

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// JSON-RPC 2.0 protocol types for plugin communication.

var nextID atomic.Int64

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Notification is a JSON-RPC 2.0 notification (no ID).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// NewRequest creates a new JSON-RPC 2.0 request with an auto-incremented ID.
func NewRequest(method string, params any) (*Request, error) {
	var raw json.RawMessage

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("plugin: marshal params: %w", err)
		}

		raw = data
	}

	return &Request{
		JSONRPC: "2.0",
		ID:      nextID.Add(1),
		Method:  method,
		Params:  raw,
	}, nil
}

// NewNotification creates a JSON-RPC 2.0 notification.
func NewNotification(method string, params any) (*Notification, error) {
	var raw json.RawMessage

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("plugin: marshal params: %w", err)
		}

		raw = data
	}

	return &Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  raw,
	}, nil
}

// message is used by the decoder to determine if a line is a request, response, or notification.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// IsRequest returns true if the message has both an ID and a method.
func (m *message) IsRequest() bool {
	return m.ID != nil && m.Method != ""
}

// IsResponse returns true if the message has an ID but no method.
func (m *message) IsResponse() bool {
	return m.ID != nil && m.Method == ""
}

// IsNotification returns true if the message has a method but no ID.
func (m *message) IsNotification() bool {
	return m.ID == nil && m.Method != ""
}
