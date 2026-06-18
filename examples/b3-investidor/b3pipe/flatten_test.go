package b3pipe

import (
	"reflect"
	"testing"
)

func TestFlattenSimpleArray(t *testing.T) {
	raw := []byte(`{"data":[{"ticker":"PETR4","qtd":100},{"ticker":"VALE3","qtd":50,"corretora":"XP"}]}`)
	header, rows, err := Flatten(raw, "data")
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	// union of keys, sorted: corretora, qtd, ticker
	wantHeader := []string{"corretora", "qtd", "ticker"}
	if !reflect.DeepEqual(header, wantHeader) {
		t.Fatalf("header = %v, want %v", header, wantHeader)
	}
	wantRows := [][]string{
		{"", "100", "PETR4"},
		{"XP", "50", "VALE3"},
	}
	if !reflect.DeepEqual(rows, wantRows) {
		t.Fatalf("rows = %v, want %v", rows, wantRows)
	}
}

func TestFlattenNestedAndTopLevelArray(t *testing.T) {
	raw := []byte(`[{"a":{"b":1},"tags":["x","y"]}]`)
	header, rows, err := Flatten(raw, "")
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if !reflect.DeepEqual(header, []string{"a.b", "tags"}) {
		t.Fatalf("header = %v", header)
	}
	if !reflect.DeepEqual(rows, [][]string{{"1", `["x","y"]`}}) {
		t.Fatalf("rows = %v", rows)
	}
}

func TestFlattenRecordPathNotArray(t *testing.T) {
	if _, _, err := Flatten([]byte(`{"data":{"x":1}}`), "data"); err == nil {
		t.Fatal("expected error: record_path is not an array")
	}
}
