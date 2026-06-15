package interaction

// Event is one JSONL record in a capture file. Kind makes each line
// self-describing; only set fields are written (omitempty).
type Event struct {
	Seq        int            `json:"seq"`
	TS         string         `json:"ts"`
	Kind       string         `json:"kind"`
	Source     string         `json:"source,omitempty"`
	Name       string         `json:"name,omitempty"`
	Input      map[string]any `json:"input,omitempty"`
	Result     string         `json:"result,omitempty"`
	OK         *bool          `json:"ok,omitempty"`
	Error      string         `json:"error,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}
