package scout

import (
	"encoding/json"
	"sync"
)

// RecordedStep represents a single recorded user interaction.
type RecordedStep struct {
	Action   string `json:"action"`
	URL      string `json:"url,omitempty"`
	Selector string `json:"selector,omitempty"`
	Text     string `json:"text,omitempty"`
}

// RecordedRunbook represents a runbook generated from recorded interactions.
type RecordedRunbook struct {
	Version string         `json:"version"`
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	URL     string         `json:"url,omitempty"`
	Steps   []RecordedStep `json:"steps,omitempty"`
}

// RecordedRunbookJSON returns the runbook as indented JSON bytes.
func (r *RecordedRunbook) RecordedRunbookJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// BridgeRecorder records bridge events and converts them to runbook steps.
type BridgeRecorder struct {
	server    *BridgeServer
	steps     []RecordedStep
	mu        sync.Mutex
	recording bool
	done      chan struct{}
}

// NewBridgeRecorder creates a new recorder attached to a bridge server.
// Returns nil if server is nil.
func NewBridgeRecorder(server *BridgeServer) *BridgeRecorder {
	if server == nil {
		return nil
	}
	return &BridgeRecorder{
		server: server,
		done:   make(chan struct{}),
	}
}

// Start begins recording bridge events and converting them to runbook steps.
// It subscribes to user.click, user.input, and navigation events.
func (r *BridgeRecorder) Start() {
	if r == nil {
		return
	}

	r.mu.Lock()
	if r.recording {
		r.mu.Unlock()
		return
	}
	r.recording = true
	r.steps = nil
	r.done = make(chan struct{})
	r.mu.Unlock()

	r.server.Subscribe(BridgeEventUserClick, func(e BridgeEvent) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.recording {
			return
		}
		selector, _ := e.Data["selector"].(string)
		if selector == "" {
			return
		}
		r.steps = append(r.steps, RecordedStep{
			Action:   "click",
			Selector: selector,
		})
	})

	r.server.Subscribe(BridgeEventUserInput, func(e BridgeEvent) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.recording {
			return
		}
		selector, _ := e.Data["selector"].(string)
		value, _ := e.Data["value"].(string)
		if selector == "" {
			return
		}
		r.steps = append(r.steps, RecordedStep{
			Action:   "type",
			Selector: selector,
			Text:     value,
		})
	})

	r.server.Subscribe(BridgeEventNavigation, func(e BridgeEvent) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.recording {
			return
		}
		url, _ := e.Data["url"].(string)
		if url == "" {
			return
		}
		r.steps = append(r.steps, RecordedStep{
			Action: "navigate",
			URL:    url,
		})
	})
}

// Stop stops recording and returns the accumulated steps.
func (r *BridgeRecorder) Stop() []RecordedStep {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return nil
	}

	r.recording = false
	close(r.done)

	out := make([]RecordedStep, len(r.steps))
	copy(out, r.steps)
	return out
}

// Steps returns the current steps without stopping the recorder.
func (r *BridgeRecorder) Steps() []RecordedStep {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]RecordedStep, len(r.steps))
	copy(out, r.steps)
	return out
}

// ToRunbook converts the recorded steps into a full automate runbook.
func (r *BridgeRecorder) ToRunbook(name, url string) *RecordedRunbook {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	steps := make([]RecordedStep, len(r.steps))
	copy(steps, r.steps)

	return &RecordedRunbook{
		Version: "1",
		Name:    name,
		Type:    "automate",
		URL:     url,
		Steps:   steps,
	}
}

// Deprecated: RecordedRecipe is an alias for RecordedRunbook. Use RecordedRunbook instead.
type RecordedRecipe = RecordedRunbook

// Deprecated: ToRecipe is an alias for ToRunbook. Use ToRunbook instead.
func (r *BridgeRecorder) ToRecipe(name, url string) *RecordedRunbook {
	return r.ToRunbook(name, url)
}

