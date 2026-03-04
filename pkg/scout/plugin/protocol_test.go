package plugin

import (
	"encoding/json"
	"testing"
)

func TestNewRequest(t *testing.T) {
	req, err := NewRequest("initialize", map[string]any{"capabilities": []string{"mcp_tool"}})
	if err != nil {
		t.Fatal(err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", req.JSONRPC, "2.0")
	}

	if req.Method != "initialize" {
		t.Errorf("method = %q, want %q", req.Method, "initialize")
	}

	if req.ID == 0 {
		t.Error("expected non-zero ID")
	}

	if req.Params == nil {
		t.Error("expected non-nil params")
	}
}

func TestNewRequest_NilParams(t *testing.T) {
	req, err := NewRequest("shutdown", nil)
	if err != nil {
		t.Fatal(err)
	}

	if req.Params != nil {
		t.Error("expected nil params")
	}
}

func TestNewNotification(t *testing.T) {
	notif, err := NewNotification("result", map[string]string{"type": "post"})
	if err != nil {
		t.Fatal(err)
	}

	if notif.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", notif.JSONRPC, "2.0")
	}

	if notif.Method != "result" {
		t.Errorf("method = %q, want %q", notif.Method, "result")
	}
}

func TestRPCError_Error(t *testing.T) {
	e := &RPCError{Code: -32601, Message: "method not found"}
	got := e.Error()

	if got != "rpc error -32601: method not found" {
		t.Errorf("Error() = %q", got)
	}
}

func TestMessage_Classification(t *testing.T) {
	id := int64(1)

	tests := []struct {
		name           string
		msg            message
		isReq, isResp, isNotif bool
	}{
		{"request", message{ID: &id, Method: "initialize"}, true, false, false},
		{"response", message{ID: &id, Result: json.RawMessage(`{}`)}, false, true, false},
		{"notification", message{Method: "result"}, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.msg.IsRequest() != tt.isReq {
				t.Errorf("IsRequest() = %v, want %v", tt.msg.IsRequest(), tt.isReq)
			}

			if tt.msg.IsResponse() != tt.isResp {
				t.Errorf("IsResponse() = %v, want %v", tt.msg.IsResponse(), tt.isResp)
			}

			if tt.msg.IsNotification() != tt.isNotif {
				t.Errorf("IsNotification() = %v, want %v", tt.msg.IsNotification(), tt.isNotif)
			}
		})
	}
}
