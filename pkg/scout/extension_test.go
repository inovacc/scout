package scout

import (
	"testing"
)

func TestWithExtensionOption(t *testing.T) {
	o := defaults()
	WithExtension("/path/to/ext1")(o)

	if len(o.extensions) != 1 || o.extensions[0] != "/path/to/ext1" {
		t.Fatalf("expected [/path/to/ext1], got %v", o.extensions)
	}

	WithExtension("/path/to/ext2", "/path/to/ext3")(o)

	if len(o.extensions) != 3 {
		t.Fatalf("expected 3 extensions, got %d", len(o.extensions))
	}

	if o.extensions[1] != "/path/to/ext2" || o.extensions[2] != "/path/to/ext3" {
		t.Fatalf("unexpected extensions: %v", o.extensions)
	}
}

func TestExtensionLaunchFlags(t *testing.T) {
	o := defaults()
	WithExtension("/ext/a", "/ext/b")(o)

	if len(o.extensions) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(o.extensions))
	}

	// Verify the extensions are stored and would produce the correct joined string.
	expected := "/ext/a,/ext/b"
	got := ""
	for i, p := range o.extensions {
		if i > 0 {
			got += ","
		}
		got += p
	}

	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
