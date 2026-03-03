package engine

import (
	"fmt"
	"sync"

	input2 "github.com/inovacc/scout/internal/engine/lib/input"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/inovacc/scout/internal/engine/lib/utils"
	"github.com/ysmood/gson"
)

// Keyboard represents the keyboard on a page, it's always related the main frame.
type Keyboard struct {
	sync.Mutex

	page *rodPage

	// pressed keys must be released before it can be pressed again
	pressed map[input2.Key]struct{}
}

func (p *rodPage) newKeyboard() *rodPage {
	p.Keyboard = &Keyboard{page: p, pressed: map[input2.Key]struct{}{}}
	return p
}

func (k *Keyboard) getModifiers() int {
	k.Lock()
	defer k.Unlock()

	return k.modifiers()
}

func (k *Keyboard) modifiers() int {
	ms := 0
	for key := range k.pressed {
		ms |= key.Modifier()
	}

	return ms
}

// Press the key down.
// To input characters that are not on the keyboard, such as Chinese or Japanese, you should
// use method like [Page.InsertText].
func (k *Keyboard) Press(key input2.Key) error {
	defer k.page.tryTrace(TraceTypeInput, "press key: "+key.Info().Code)()

	k.page.browser.trySlowMotion()

	k.Lock()
	defer k.Unlock()

	k.pressed[key] = struct{}{}

	return key.Encode(proto2.InputDispatchKeyEventTypeKeyDown, k.modifiers()).Call(k.page)
}

// Release the key.
func (k *Keyboard) Release(key input2.Key) error {
	defer k.page.tryTrace(TraceTypeInput, "release key: "+key.Info().Code)()

	k.Lock()
	defer k.Unlock()

	if _, has := k.pressed[key]; !has {
		return nil
	}

	delete(k.pressed, key)

	return key.Encode(proto2.InputDispatchKeyEventTypeKeyUp, k.modifiers()).Call(k.page)
}

// Type releases the key after the press.
func (k *Keyboard) Type(keys ...input2.Key) (err error) {
	for _, key := range keys {
		err = k.Press(key)
		if err != nil {
			return
		}

		err = k.Release(key)
		if err != nil {
			return
		}
	}

	return
}

// KeyActionType enum.
type KeyActionType int

// KeyActionTypes.
const (
	KeyActionPress KeyActionType = iota
	KeyActionRelease
	KeyActionTypeKey
)

// KeyAction to perform.
type KeyAction struct {
	Type KeyActionType
	Key  input2.Key
}

// KeyActions to simulate.
type KeyActions struct {
	keyboard *Keyboard

	Actions []KeyAction
}

// KeyActions simulates the type actions on a physical keyboard.
// Useful when input shortcuts like ctrl+enter .
func (p *rodPage) KeyActions() *KeyActions {
	return &KeyActions{keyboard: p.Keyboard}
}

// Press keys is guaranteed to have a release at the end of actions.
func (ka *KeyActions) Press(keys ...input2.Key) *KeyActions {
	for _, key := range keys {
		ka.Actions = append(ka.Actions, KeyAction{KeyActionPress, key})
	}

	return ka
}

// Release keys.
func (ka *KeyActions) Release(keys ...input2.Key) *KeyActions {
	for _, key := range keys {
		ka.Actions = append(ka.Actions, KeyAction{KeyActionRelease, key})
	}

	return ka
}

// Type will release the key immediately after the pressing.
func (ka *KeyActions) Type(keys ...input2.Key) *KeyActions {
	for _, key := range keys {
		ka.Actions = append(ka.Actions, KeyAction{KeyActionTypeKey, key})
	}

	return ka
}

// Do the actions.
func (ka *KeyActions) Do() (err error) {
	for _, a := range ka.balance() {
		switch a.Type {
		case KeyActionPress:
			err = ka.keyboard.Press(a.Key)
		case KeyActionRelease:
			err = ka.keyboard.Release(a.Key)
		case KeyActionTypeKey:
			err = ka.keyboard.Type(a.Key)
		}

		if err != nil {
			return
		}
	}

	return
}

// Make sure there's at least one release after the presses, such as:
//
//	p1,p2,p1,r1 => p1,p2,p1,r1,r2
func (ka *KeyActions) balance() []KeyAction {
	actions := ka.Actions

	h := map[input2.Key]bool{}

	for _, a := range actions {
		switch a.Type {
		case KeyActionPress:
			h[a.Key] = true
		case KeyActionRelease, KeyActionTypeKey:
			h[a.Key] = false
		}
	}

	for key, needRelease := range h {
		if needRelease {
			actions = append(actions, KeyAction{KeyActionRelease, key})
		}
	}

	return actions
}

// InsertText is like pasting text into the page.
func (p *rodPage) InsertText(text string) error {
	defer p.tryTrace(TraceTypeInput, "insert text "+text)()

	p.browser.trySlowMotion()

	err := proto2.InputInsertText{Text: text}.Call(p)

	return err
}

// Mouse represents the mouse on a page, it's always related the main frame.
type Mouse struct {
	sync.Mutex

	page *rodPage

	id string // mouse svg dom element id

	pos proto2.Point

	// the buttons is currently being pressed, reflects the press order
	buttons []proto2.InputMouseButton
}

func (p *rodPage) newMouse() *rodPage {
	p.Mouse = &Mouse{page: p, id: utils.RandString(8)}
	return p
}

// Position of current cursor.
func (m *Mouse) Position() proto2.Point {
	m.Lock()
	defer m.Unlock()

	return m.pos
}

// MoveTo the absolute position.
func (m *Mouse) MoveTo(p proto2.Point) error {
	m.Lock()
	defer m.Unlock()

	button, buttons := input2.EncodeMouseButton(m.buttons)

	m.page.browser.trySlowMotion()

	err := proto2.InputDispatchMouseEvent{
		Type:      proto2.InputDispatchMouseEventTypeMouseMoved,
		X:         p.X,
		Y:         p.Y,
		Button:    button,
		Buttons:   gson.Int(buttons),
		Modifiers: m.page.Keyboard.getModifiers(),
	}.Call(m.page)
	if err != nil {
		return err
	}

	// to make sure set only when call is successful
	m.pos = p

	if m.page.browser.trace {
		if !m.updateMouseTracer() {
			m.initMouseTracer()
			m.updateMouseTracer()
		}
	}

	return nil
}

// MoveAlong the guide function.
// Every time the guide function is called it should return the next mouse position, return true to stop.
// Read the source code of [Mouse.MoveLinear] as an example to use this method.
func (m *Mouse) MoveAlong(guide func() (proto2.Point, bool)) error {
	for {
		p, stop := guide()
		if stop {
			return m.MoveTo(p)
		}

		err := m.MoveTo(p)
		if err != nil {
			return err
		}
	}
}

// MoveLinear to the absolute position with the given steps.
// Such as move from (0,0) to (6,6) with 3 steps, the mouse will first move to (2,2) then (4,4) then (6,6).
func (m *Mouse) MoveLinear(to proto2.Point, steps int) error {
	p := m.Position()
	step := to.Minus(p).Scale(1 / float64(steps))
	count := 0

	return m.MoveAlong(func() (proto2.Point, bool) {
		count++
		if count == steps {
			return to, true
		}

		p = p.Add(step)

		return p, false
	})
}

// Scroll the relative offset with specified steps.
func (m *Mouse) Scroll(offsetX, offsetY float64, steps int) error {
	m.Lock()
	defer m.Unlock()

	defer m.page.tryTrace(TraceTypeInput, fmt.Sprintf("scroll (%.2f, %.2f)", offsetX, offsetY))()

	m.page.browser.trySlowMotion()

	if steps < 1 {
		steps = 1
	}

	button, buttons := input2.EncodeMouseButton(m.buttons)

	stepX := offsetX / float64(steps)
	stepY := offsetY / float64(steps)

	for i := 0; i < steps; i++ {
		err := proto2.InputDispatchMouseEvent{
			Type:      proto2.InputDispatchMouseEventTypeMouseWheel,
			Button:    button,
			Buttons:   gson.Int(buttons),
			Modifiers: m.page.Keyboard.getModifiers(),
			DeltaX:    stepX,
			DeltaY:    stepY,
			X:         m.pos.X,
			Y:         m.pos.Y,
		}.Call(m.page)
		if err != nil {
			return err
		}
	}

	return nil
}

// Down holds the button down.
func (m *Mouse) Down(button proto2.InputMouseButton, clickCount int) error {
	m.Lock()
	defer m.Unlock()

	toButtons := append(append([]proto2.InputMouseButton{}, m.buttons...), button)

	_, buttons := input2.EncodeMouseButton(toButtons)

	err := proto2.InputDispatchMouseEvent{
		Type:       proto2.InputDispatchMouseEventTypeMousePressed,
		Button:     button,
		Buttons:    gson.Int(buttons),
		ClickCount: clickCount,
		Modifiers:  m.page.Keyboard.getModifiers(),
		X:          m.pos.X,
		Y:          m.pos.Y,
	}.Call(m.page)
	if err != nil {
		return err
	}

	m.buttons = toButtons

	return nil
}

// Up releases the button.
func (m *Mouse) Up(button proto2.InputMouseButton, clickCount int) error {
	m.Lock()
	defer m.Unlock()

	toButtons := []proto2.InputMouseButton{}

	for _, btn := range m.buttons {
		if btn == button {
			continue
		}

		toButtons = append(toButtons, btn)
	}

	_, buttons := input2.EncodeMouseButton(toButtons)

	err := proto2.InputDispatchMouseEvent{
		Type:       proto2.InputDispatchMouseEventTypeMouseReleased,
		Button:     button,
		Buttons:    gson.Int(buttons),
		ClickCount: clickCount,
		Modifiers:  m.page.Keyboard.getModifiers(),
		X:          m.pos.X,
		Y:          m.pos.Y,
	}.Call(m.page)
	if err != nil {
		return err
	}

	m.buttons = toButtons

	return nil
}

// Click the button. It's the combination of [Mouse.Down] and [Mouse.Up].
func (m *Mouse) Click(button proto2.InputMouseButton, clickCount int) error {
	m.page.browser.trySlowMotion()

	err := m.Down(button, clickCount)
	if err != nil {
		return err
	}

	return m.Up(button, clickCount)
}

// Touch presents a touch device, such as a hand with fingers, each finger is a [proto.InputTouchPoint].
// Touch events is stateless, we use the struct here only as a namespace to make the API style unified.
type Touch struct {
	page *rodPage
}

func (p *rodPage) newTouch() *rodPage {
	p.Touch = &Touch{page: p}
	return p
}

// Start a touch action.
func (t *Touch) Start(points ...*proto2.InputTouchPoint) error {
	// TODO: https://crbug.com/613219
	_ = t.page.WaitRepaint()
	_ = t.page.WaitRepaint()

	return proto2.InputDispatchTouchEvent{
		Type:        proto2.InputDispatchTouchEventTypeTouchStart,
		TouchPoints: points,
		Modifiers:   t.page.Keyboard.getModifiers(),
	}.Call(t.page)
}

// Move touch points. Use the [proto.InputTouchPoint.ID] (Touch.identifier) to track points.
// Doc: https://developer.mozilla.org/en-US/docs/Web/API/Touch_events
func (t *Touch) Move(points ...*proto2.InputTouchPoint) error {
	return proto2.InputDispatchTouchEvent{
		Type:        proto2.InputDispatchTouchEventTypeTouchMove,
		TouchPoints: points,
		Modifiers:   t.page.Keyboard.getModifiers(),
	}.Call(t.page)
}

// End touch action.
func (t *Touch) End() error {
	return proto2.InputDispatchTouchEvent{
		Type:        proto2.InputDispatchTouchEventTypeTouchEnd,
		TouchPoints: []*proto2.InputTouchPoint{},
		Modifiers:   t.page.Keyboard.getModifiers(),
	}.Call(t.page)
}

// Cancel touch action.
func (t *Touch) Cancel() error {
	return proto2.InputDispatchTouchEvent{
		Type:        proto2.InputDispatchTouchEventTypeTouchCancel,
		TouchPoints: []*proto2.InputTouchPoint{},
		Modifiers:   t.page.Keyboard.getModifiers(),
	}.Call(t.page)
}

// Tap dispatches a touchstart and touchend event.
func (t *Touch) Tap(x, y float64) error {
	defer t.page.tryTrace(TraceTypeInput, "touch")()

	t.page.browser.trySlowMotion()

	p := &proto2.InputTouchPoint{X: x, Y: y}

	err := t.Start(p)
	if err != nil {
		return err
	}

	return t.End()
}
