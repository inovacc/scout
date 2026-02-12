package scout

import (
	"encoding/json"
	"testing"
)

func TestEvalResultString(t *testing.T) {
	tests := []struct {
		name string
		r    *EvalResult
		want string
	}{
		{"nil result", nil, ""},
		{"nil value", &EvalResult{Value: nil}, ""},
		{"string value", &EvalResult{Value: "hello"}, "hello"},
		{"json number", &EvalResult{Value: json.Number("42")}, "42"},
		{"bool true", &EvalResult{Value: true}, "true"},
		{"bool false", &EvalResult{Value: false}, "false"},
		{"other type", &EvalResult{Value: []int{1, 2}}, "[1 2]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvalResultInt(t *testing.T) {
	tests := []struct {
		name string
		r    *EvalResult
		want int
	}{
		{"nil result", nil, 0},
		{"nil value", &EvalResult{Value: nil}, 0},
		{"json number", &EvalResult{Value: json.Number("42")}, 42},
		{"float64", &EvalResult{Value: float64(99)}, 99},
		{"string number", &EvalResult{Value: "123"}, 123},
		{"non-numeric string", &EvalResult{Value: "abc"}, 0},
		{"other type", &EvalResult{Value: true}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Int(); got != tt.want {
				t.Errorf("Int() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEvalResultFloat(t *testing.T) {
	tests := []struct {
		name string
		r    *EvalResult
		want float64
	}{
		{"nil result", nil, 0},
		{"nil value", &EvalResult{Value: nil}, 0},
		{"json number", &EvalResult{Value: json.Number("3.14")}, 3.14},
		{"float64", &EvalResult{Value: float64(2.718)}, 2.718},
		{"string float", &EvalResult{Value: "1.5"}, 1.5},
		{"non-numeric string", &EvalResult{Value: "abc"}, 0},
		{"other type", &EvalResult{Value: true}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Float(); got != tt.want {
				t.Errorf("Float() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestEvalResultBool(t *testing.T) {
	tests := []struct {
		name string
		r    *EvalResult
		want bool
	}{
		{"nil result", nil, false},
		{"nil value", &EvalResult{Value: nil}, false},
		{"bool true", &EvalResult{Value: true}, true},
		{"bool false", &EvalResult{Value: false}, false},
		{"string true", &EvalResult{Value: "true"}, true},
		{"string false", &EvalResult{Value: "false"}, false},
		{"json number non-zero", &EvalResult{Value: json.Number("1")}, true},
		{"json number zero", &EvalResult{Value: json.Number("0")}, false},
		{"other type", &EvalResult{Value: []int{1}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Bool(); got != tt.want {
				t.Errorf("Bool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalResultIsNull(t *testing.T) {
	tests := []struct {
		name string
		r    *EvalResult
		want bool
	}{
		{"nil result", nil, true},
		{"undefined type", &EvalResult{Type: "undefined"}, true},
		{"null subtype", &EvalResult{Subtype: "null"}, true},
		{"nil value", &EvalResult{Value: nil}, true},
		{"string value", &EvalResult{Value: "hello"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsNull(); got != tt.want {
				t.Errorf("IsNull() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalResultJSON(t *testing.T) {
	// nil result
	var r *EvalResult
	if got := r.JSON(); got != nil {
		t.Errorf("JSON() on nil = %v, want nil", got)
	}

	// with rawJSON
	r = &EvalResult{rawJSON: []byte(`{"key":"val"}`)}
	if got := string(r.JSON()); got != `{"key":"val"}` {
		t.Errorf("JSON() with rawJSON = %q, want %q", got, `{"key":"val"}`)
	}

	// without rawJSON, marshals Value
	r = &EvalResult{Value: "hello"}
	if got := string(r.JSON()); got != `"hello"` {
		t.Errorf("JSON() from Value = %q, want %q", got, `"hello"`)
	}

	// numeric value
	r = &EvalResult{Value: json.Number("42")}
	if got := string(r.JSON()); got != `42` {
		t.Errorf("JSON() numeric = %q, want %q", got, `42`)
	}
}

func TestEvalResultDecode(t *testing.T) {
	// nil result
	var r *EvalResult

	var target map[string]string

	if err := r.Decode(&target); err == nil {
		t.Error("Decode() on nil should return error")
	}

	// valid decode
	r = &EvalResult{Value: map[string]any{"name": "test", "age": "25"}}

	var m map[string]any
	if err := r.Decode(&m); err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if m["name"] != "test" {
		t.Errorf("Decode() name = %v, want %q", m["name"], "test")
	}

	// with rawJSON
	r = &EvalResult{rawJSON: []byte(`{"x":1,"y":2}`)}

	var coords map[string]int
	if err := r.Decode(&coords); err != nil {
		t.Fatalf("Decode() rawJSON error: %v", err)
	}

	if coords["x"] != 1 || coords["y"] != 2 {
		t.Errorf("Decode() coords = %v, want x=1 y=2", coords)
	}
}
