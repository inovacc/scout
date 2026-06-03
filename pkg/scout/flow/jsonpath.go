package flow

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ExtractPath walks a dotted JSON path against data and returns the value as a
// string. Supported syntax: a leading "$." then dot-separated keys; a numeric
// segment indexes an array (e.g. "$.items.1.sku"). Scalars stringify naturally;
// objects/arrays return compact JSON. A missing key or out-of-range index errors.
func ExtractPath(data []byte, path string) (string, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return "", fmt.Errorf("scout: flow: jsonpath: parse: %w", err)
	}
	segs := strings.Split(strings.TrimPrefix(path, "$."), ".")
	cur := root
	for _, seg := range segs {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return "", fmt.Errorf("scout: flow: jsonpath: key %q not found", seg)
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(node) {
				return "", fmt.Errorf("scout: flow: jsonpath: index %q out of range", seg)
			}
			cur = node[i]
		default:
			return "", fmt.Errorf("scout: flow: jsonpath: cannot descend into %q", seg)
		}
	}
	return stringify(cur)
}

func stringify(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case float64:
		// JSON numbers decode to float64; render integers without a trailing .0.
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10), nil
		}
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(t), nil
	case nil:
		return "", nil
	default: // object or array → compact JSON
		b, err := json.Marshal(t)
		if err != nil {
			return "", fmt.Errorf("scout: flow: jsonpath: marshal value: %w", err)
		}
		return string(b), nil
	}
}
