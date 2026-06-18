package b3pipe

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Flatten parses raw JSON, walks recordPath (dot-separated; "" = root) to a
// JSON array, and flattens each element into a row. Header is the sorted union
// of dotted keys across all records. Nested arrays/objects without scalar leaves
// are emitted as compact JSON strings.
func Flatten(raw []byte, recordPath string) (header []string, rows [][]string, err error) {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, nil, fmt.Errorf("b3: flatten: parse json: %w", err)
	}
	node := root
	if recordPath != "" {
		for _, key := range strings.Split(recordPath, ".") {
			m, ok := node.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("b3: flatten: record_path %q: %q is not an object", recordPath, key)
			}
			node, ok = m[key]
			if !ok {
				return nil, nil, fmt.Errorf("b3: flatten: record_path %q: key %q not found", recordPath, key)
			}
		}
	}
	arr, ok := node.([]any)
	if !ok {
		return nil, nil, fmt.Errorf("b3: flatten: record_path %q does not point to an array", recordPath)
	}

	flat := make([]map[string]string, len(arr))
	keySet := map[string]struct{}{}
	for i, el := range arr {
		m := map[string]string{}
		flattenValue("", el, m)
		flat[i] = m
		for k := range m {
			keySet[k] = struct{}{}
		}
	}
	header = make([]string, 0, len(keySet))
	for k := range keySet {
		header = append(header, k)
	}
	sort.Strings(header)

	rows = make([][]string, len(flat))
	for i, m := range flat {
		row := make([]string, len(header))
		for j, h := range header {
			row[j] = m[h] // missing key -> ""
		}
		rows[i] = row
	}
	return header, rows, nil
}

func flattenValue(prefix string, v any, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, sub := range t {
			child := k
			if prefix != "" {
				child = prefix + "." + k
			}
			flattenValue(child, sub, out)
		}
	case []any:
		b, _ := json.Marshal(t) // nested array -> compact JSON cell
		out[prefix] = string(b)
	case float64:
		out[prefix] = strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		out[prefix] = strconv.FormatBool(t)
	case nil:
		out[prefix] = ""
	default:
		out[prefix] = fmt.Sprintf("%v", t)
	}
}
