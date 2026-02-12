package scout

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// EvalResult wraps the result of a JavaScript evaluation.
type EvalResult struct {
	Type    string
	Subtype string
	Value   any
	rawJSON []byte
}

// String returns the result as a string.
func (r *EvalResult) String() string {
	if r == nil || r.Value == nil {
		return ""
	}

	switch v := r.Value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Int returns the result as an int.
func (r *EvalResult) Int() int {
	if r == nil || r.Value == nil {
		return 0
	}

	switch v := r.Value.(type) {
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

// Float returns the result as a float64.
func (r *EvalResult) Float() float64 {
	if r == nil || r.Value == nil {
		return 0
	}

	switch v := r.Value.(type) {
	case json.Number:
		f, _ := v.Float64()
		return f
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

// Bool returns the result as a bool.
func (r *EvalResult) Bool() bool {
	if r == nil || r.Value == nil {
		return false
	}

	switch v := r.Value.(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	case json.Number:
		return v.String() != "0"
	default:
		return false
	}
}

// IsNull returns true if the result is null or undefined.
func (r *EvalResult) IsNull() bool {
	if r == nil {
		return true
	}

	return r.Type == "undefined" || r.Subtype == "null" || r.Value == nil
}

// JSON returns the raw JSON representation of the value.
func (r *EvalResult) JSON() []byte {
	if r == nil {
		return nil
	}

	if r.rawJSON != nil {
		return r.rawJSON
	}

	b, err := json.Marshal(r.Value)
	if err != nil {
		return nil
	}

	return b
}

// Decode unmarshals the result into the provided target.
func (r *EvalResult) Decode(target any) error {
	if r == nil {
		return fmt.Errorf("scout: eval result is nil")
	}

	data := r.JSON()
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("scout: decode eval result: %w", err)
	}

	return nil
}
