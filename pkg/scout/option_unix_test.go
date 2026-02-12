//go:build !windows

package scout

import "testing"

func TestWithXvfb(t *testing.T) {
	o := defaults()

	if o.xvfb {
		t.Error("default xvfb should be false")
	}

	WithXvfb()(o)

	if !o.xvfb {
		t.Error("WithXvfb should set xvfb=true")
	}

	if len(o.xvfbArgs) != 0 {
		t.Errorf("WithXvfb() with no args should have empty xvfbArgs, got %v", o.xvfbArgs)
	}

	o2 := defaults()
	WithXvfb("-screen", "0", "1280x1024x24")(o2)

	if !o2.xvfb {
		t.Error("WithXvfb should set xvfb=true")
	}

	if len(o2.xvfbArgs) != 3 {
		t.Errorf("WithXvfb with args should store 3 args, got %d", len(o2.xvfbArgs))
	}

	expected := []string{"-screen", "0", "1280x1024x24"}
	for i, arg := range o2.xvfbArgs {
		if arg != expected[i] {
			t.Errorf("xvfbArgs[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}
