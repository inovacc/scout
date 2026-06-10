package input

import (
	"testing"

	"github.com/inovacc/scout/internal/engine/lib/proto"
)

// TestKeyInfo verifies that well-known keys resolve to the expected KeyInfo.
func TestKeyInfo(t *testing.T) {
	tests := []struct {
		name     string
		key      Key
		wantKey  string
		wantCode string
		wantVK   int
		wantLoc  int
	}{
		{"letter a", KeyA, "a", "KeyA", 65, 0},
		{"digit 1", Digit1, "1", "Digit1", 49, 0},
		{"shift left", ShiftLeft, "Shift", "ShiftLeft", 16, 1},
		{"shift right", ShiftRight, "Shift", "ShiftRight", 16, 2},
		{"control left", ControlLeft, "Control", "ControlLeft", 17, 1},
		{"f1 function", F1, "F1", "F1", 112, 0},
		{"numpad add", NumpadAdd, "+", "NumpadAdd", 107, 3},
		{"space", Space, " ", "Space", 32, 0},
		{"enter", Enter, "\r", "Enter", 13, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := tt.key.Info()
			if info.Key != tt.wantKey {
				t.Errorf("Key = %q, want %q", info.Key, tt.wantKey)
			}
			if info.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", info.Code, tt.wantCode)
			}
			if info.KeyCode != tt.wantVK {
				t.Errorf("KeyCode = %d, want %d", info.KeyCode, tt.wantVK)
			}
			if info.Location != tt.wantLoc {
				t.Errorf("Location = %d, want %d", info.Location, tt.wantLoc)
			}
		})
	}
}

// TestKeyInfoFromShiftedMap verifies a shifted symbol resolves via keyMapShifted.
func TestKeyInfoFromShiftedMap(t *testing.T) {
	// '!' is the shifted form of '1' and lives only in keyMapShifted.
	info := Key('!').Info()
	if info.Key != "!" {
		t.Fatalf("shifted Key = %q, want %q", info.Key, "!")
	}
	if info.Code != "Digit1" {
		t.Errorf("shifted Code = %q, want %q", info.Code, "Digit1")
	}
	if info.KeyCode != 49 {
		t.Errorf("shifted KeyCode = %d, want 49", info.KeyCode)
	}
}

// TestKeyInfoPanicsOnUnknown verifies Info panics for an unregistered key.
func TestKeyInfoPanicsOnUnknown(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unknown key, got none")
		}
		if msg, ok := r.(string); !ok || msg != "key not defined" {
			t.Errorf("panic value = %v, want %q", r, "key not defined")
		}
	}()

	// A rune that was never registered by any AddKey call.
	_ = Key(0xE000).Info()
}

// TestKeyShift verifies the shifted-variant lookup for known and unknown keys.
func TestKeyShift(t *testing.T) {
	tests := []struct {
		name      string
		key       Key
		wantShift Key
		wantHas   bool
	}{
		{"digit 1 -> bang", Digit1, Key('!'), true},
		{"letter a -> A", KeyA, Key('A'), true},
		{"semicolon -> colon", Semicolon, Key(':'), true},
		{"space has no shift", Space, 0, false},
		{"unregistered rune", Key(0xE001), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, has := tt.key.Shift()
			if has != tt.wantHas {
				t.Fatalf("has = %v, want %v", has, tt.wantHas)
			}
			if has && got != tt.wantShift {
				t.Errorf("shift = %q, want %q", rune(got), rune(tt.wantShift))
			}
		})
	}
}

// TestKeyPrintable verifies single-character keys are printable, others not.
func TestKeyPrintable(t *testing.T) {
	tests := []struct {
		name string
		key  Key
		want bool
	}{
		{"letter a printable", KeyA, true},
		{"digit 0 printable", Digit0, true},
		{"space printable", Space, true},
		{"enter printable (CR)", Enter, true},
		{"escape not printable", Escape, false},
		{"f1 not printable", F1, false},
		{"shift not printable", ShiftLeft, false},
		{"arrow up not printable", ArrowUp, false},
		{"backspace not printable", Backspace, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.key.Printable(); got != tt.want {
				t.Errorf("Printable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestKeyModifier verifies KeyCode-to-modifier mapping across all branches.
func TestKeyModifier(t *testing.T) {
	tests := []struct {
		name string
		key  Key
		want int
	}{
		{"alt left", AltLeft, ModifierAlt},
		{"alt right", AltRight, ModifierAlt},
		{"control left", ControlLeft, ModifierControl},
		{"control right", ControlRight, ModifierControl},
		{"meta left", MetaLeft, ModifierMeta},
		{"meta right", MetaRight, ModifierMeta},
		{"shift left", ShiftLeft, ModifierShift},
		{"shift right", ShiftRight, ModifierShift},
		{"letter a no modifier", KeyA, 0},
		{"f1 no modifier", F1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.key.Modifier(); got != tt.want {
				t.Errorf("Modifier() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestModifierConstants pins the modifier bit values.
func TestModifierConstants(t *testing.T) {
	if ModifierAlt != 1 || ModifierControl != 2 || ModifierMeta != 4 || ModifierShift != 8 {
		t.Fatalf("modifier constants drifted: alt=%d ctrl=%d meta=%d shift=%d",
			ModifierAlt, ModifierControl, ModifierMeta, ModifierShift)
	}
}

// TestEncodePrintableKeyDown verifies a printable key encodes text + keyDown type.
func TestEncodePrintableKeyDown(t *testing.T) {
	e := KeyA.Encode(proto.InputDispatchKeyEventTypeKeyDown, ModifierShift)

	if e.Type != proto.InputDispatchKeyEventTypeKeyDown {
		t.Errorf("Type = %q, want keyDown", e.Type)
	}
	if e.Text != "a" {
		t.Errorf("Text = %q, want %q", e.Text, "a")
	}
	if e.UnmodifiedText != "a" {
		t.Errorf("UnmodifiedText = %q, want %q", e.UnmodifiedText, "a")
	}
	if e.WindowsVirtualKeyCode != 65 {
		t.Errorf("WindowsVirtualKeyCode = %d, want 65", e.WindowsVirtualKeyCode)
	}
	if e.Code != "KeyA" {
		t.Errorf("Code = %q, want KeyA", e.Code)
	}
	if e.Modifiers != ModifierShift {
		t.Errorf("Modifiers = %d, want %d", e.Modifiers, ModifierShift)
	}
	if e.IsKeypad {
		t.Error("IsKeypad = true, want false for a regular key")
	}
	if e.Location == nil {
		t.Fatal("Location = nil, want non-nil for location 0")
	}
	if *e.Location != 0 {
		t.Errorf("*Location = %d, want 0", *e.Location)
	}
}

// TestEncodeNonPrintableKeyDownPromotesToRaw verifies a non-printable keyDown
// is promoted to rawKeyDown and carries no text.
func TestEncodeNonPrintableKeyDownPromotesToRaw(t *testing.T) {
	e := Escape.Encode(proto.InputDispatchKeyEventTypeKeyDown, 0)

	if e.Type != proto.InputDispatchKeyEventTypeRawKeyDown {
		t.Errorf("Type = %q, want rawKeyDown", e.Type)
	}
	if e.Text != "" {
		t.Errorf("Text = %q, want empty", e.Text)
	}
	if e.UnmodifiedText != "" {
		t.Errorf("UnmodifiedText = %q, want empty", e.UnmodifiedText)
	}
	if e.Key != "Escape" {
		t.Errorf("Key = %q, want Escape", e.Key)
	}
}

// TestEncodeNonPrintableKeyUpStaysKeyUp verifies the promotion only applies to
// keyDown, never to keyUp.
func TestEncodeNonPrintableKeyUpStaysKeyUp(t *testing.T) {
	e := Escape.Encode(proto.InputDispatchKeyEventTypeKeyUp, 0)

	if e.Type != proto.InputDispatchKeyEventTypeKeyUp {
		t.Errorf("Type = %q, want keyUp (no promotion on keyUp)", e.Type)
	}
}

// TestEncodeKeypadLocation verifies numpad keys (location 3) clear Location and
// set IsKeypad.
func TestEncodeKeypadLocation(t *testing.T) {
	e := Numpad5.Encode(proto.InputDispatchKeyEventTypeKeyDown, 0)

	if e.Location != nil {
		t.Errorf("Location = %v, want nil for keypad", *e.Location)
	}
	if !e.IsKeypad {
		t.Error("IsKeypad = false, want true for numpad key")
	}
	// Numpad5 has key "5" which is printable, so text should be set.
	if e.Text != "5" {
		t.Errorf("Text = %q, want %q", e.Text, "5")
	}
}

// TestEncodeSidedKeyLocation verifies a left/right modifier carries its location.
func TestEncodeSidedKeyLocation(t *testing.T) {
	e := ShiftRight.Encode(proto.InputDispatchKeyEventTypeKeyDown, 0)

	if e.Location == nil {
		t.Fatal("Location = nil, want non-nil for ShiftRight")
	}
	if *e.Location != 2 {
		t.Errorf("*Location = %d, want 2", *e.Location)
	}
	if e.IsKeypad {
		t.Error("IsKeypad = true, want false for ShiftRight")
	}
}

// TestAddKeyMultiCharComputesSyntheticKey verifies AddKey returns a synthetic
// key value for multi-character names: keyCode + (location+1)*256.
func TestAddKeyMultiCharComputesSyntheticKey(t *testing.T) {
	// Re-derive the formula and confirm the resolved Info round-trips.
	keyCode, location := 999, 1
	k := AddKey("SyntheticTestKey", "", "SyntheticTestCode", keyCode, location)

	want := Key(keyCode + (location+1)*256)
	if k != want {
		t.Fatalf("AddKey synthetic key = %d, want %d", k, want)
	}

	info := k.Info()
	if info.Key != "SyntheticTestKey" {
		t.Errorf("Info.Key = %q, want SyntheticTestKey", info.Key)
	}
	if info.Code != "SyntheticTestCode" {
		t.Errorf("Info.Code = %q, want SyntheticTestCode", info.Code)
	}
	if info.KeyCode != keyCode {
		t.Errorf("Info.KeyCode = %d, want %d", info.KeyCode, keyCode)
	}
}

// TestAddKeySingleCharReturnsRune verifies single-BYTE keys map to their rune.
// AddKey uses len(key) == 1 (byte length), so the input must be one byte.
func TestAddKeySingleCharReturnsRune(t *testing.T) {
	// \x01 (SOH) is a single byte and is not registered by any layout key.
	k := AddKey("\x01", "", "TestSohCode", 777, 0)
	if k != Key('\x01') {
		t.Fatalf("single-byte AddKey = %d, want %d", k, Key('\x01'))
	}
	if got := k.Info().Code; got != "TestSohCode" {
		t.Errorf("Info.Code = %q, want TestSohCode", got)
	}
}

// TestAddKeyMultiByteCharFallsThrough verifies that a multi-byte UTF-8 char
// fails the len==1 byte check and is treated as a synthetic (non-rune) key.
func TestAddKeyMultiByteCharFallsThrough(t *testing.T) {
	// "§" is two bytes in UTF-8, so len(key) != 1 and AddKey takes the
	// synthetic-key branch: keyCode + (location+1)*256.
	k := AddKey("§", "", "SectionCode", 777, 0)
	wantSynthetic := Key(777 + (0+1)*256)
	if k != wantSynthetic {
		t.Fatalf("multi-byte AddKey = %d, want synthetic %d", k, wantSynthetic)
	}
	if k.Info().Code != "SectionCode" {
		t.Errorf("Info.Code = %q, want SectionCode", k.Info().Code)
	}
}

// TestAddKeyDuplicateSingleCharFallsThrough verifies that re-adding an existing
// single-byte key does NOT clobber the first registration and instead returns a
// synthetic key for the second one.
func TestAddKeyDuplicateSingleCharFallsThrough(t *testing.T) {
	first := AddKey("\x02", "", "FirstCode", 800, 0)
	if first != Key('\x02') {
		t.Fatalf("first AddKey = %d, want rune %d", first, Key('\x02'))
	}

	// Second add with same single byte + different code/keyCode: the rune is
	// already taken, so AddKey must fall through to the synthetic-key branch.
	second := AddKey("\x02", "", "SecondCode", 801, 0)
	wantSynthetic := Key(801 + (0+1)*256)
	if second != wantSynthetic {
		t.Fatalf("duplicate AddKey = %d, want synthetic %d", second, wantSynthetic)
	}

	// First registration is preserved.
	if Key('\x02').Info().Code != "FirstCode" {
		t.Errorf("original key clobbered: Code = %q, want FirstCode", Key('\x02').Info().Code)
	}
	// Synthetic key resolves to the second registration.
	if second.Info().Code != "SecondCode" {
		t.Errorf("synthetic key Code = %q, want SecondCode", second.Info().Code)
	}
}

// TestEncodeMouseButton exercises flag accumulation and the first-button rule.
func TestEncodeMouseButton(t *testing.T) {
	tests := []struct {
		name     string
		buttons  []proto.InputMouseButton
		wantBtn  proto.InputMouseButton
		wantFlag int
	}{
		{"empty -> none", nil, proto.InputMouseButton("none"), 0},
		{"left only", []proto.InputMouseButton{proto.InputMouseButtonLeft}, proto.InputMouseButtonLeft, 1},
		{"right only", []proto.InputMouseButton{proto.InputMouseButtonRight}, proto.InputMouseButtonRight, 2},
		{"middle only", []proto.InputMouseButton{proto.InputMouseButtonMiddle}, proto.InputMouseButtonMiddle, 4},
		{
			"left+right -> first is left, flags OR",
			[]proto.InputMouseButton{proto.InputMouseButtonLeft, proto.InputMouseButtonRight},
			proto.InputMouseButtonLeft,
			3,
		},
		{
			"all five accumulate",
			[]proto.InputMouseButton{
				proto.InputMouseButtonLeft,
				proto.InputMouseButtonRight,
				proto.InputMouseButtonMiddle,
				proto.InputMouseButtonBack,
				proto.InputMouseButtonForward,
			},
			proto.InputMouseButtonLeft,
			1 | 2 | 4 | 8 | 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			btn, flag := EncodeMouseButton(tt.buttons)
			if btn != tt.wantBtn {
				t.Errorf("button = %q, want %q", btn, tt.wantBtn)
			}
			if flag != tt.wantFlag {
				t.Errorf("flag = %d, want %d", flag, tt.wantFlag)
			}
		})
	}
}

// TestEncodeMouseButtonDuplicateIsIdempotent verifies OR-ing the same button
// twice does not double its flag.
func TestEncodeMouseButtonDuplicateIsIdempotent(t *testing.T) {
	btn, flag := EncodeMouseButton([]proto.InputMouseButton{
		proto.InputMouseButtonLeft,
		proto.InputMouseButtonLeft,
	})
	if btn != proto.InputMouseButtonLeft {
		t.Errorf("button = %q, want left", btn)
	}
	if flag != 1 {
		t.Errorf("flag = %d, want 1 (idempotent OR)", flag)
	}
}
