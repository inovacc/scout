package plugin

import (
	"testing"
)

func TestSinkProxy_Name(t *testing.T) {
	proxy := NewSinkProxy(nil, "test-sink")

	if proxy.Name() != "test-sink" {
		t.Errorf("Name() = %q, want test-sink", proxy.Name())
	}
}

func TestListSinks(t *testing.T) {
	raw := []byte(`[{"name":"s3","description":"Write to S3"},{"name":"webhook","description":"POST to URL"}]`)

	sinks, err := ListSinks(raw)
	if err != nil {
		t.Fatalf("ListSinks: %v", err)
	}

	if len(sinks) != 2 {
		t.Fatalf("len = %d, want 2", len(sinks))
	}

	if sinks[0].Name != "s3" {
		t.Errorf("sinks[0].Name = %q, want s3", sinks[0].Name)
	}
}

func TestListSinks_Invalid(t *testing.T) {
	_, err := ListSinks([]byte(`invalid`))
	if err == nil {
		t.Error("expected error on invalid JSON")
	}
}
