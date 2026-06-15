package engine

import (
	"fmt"
	"strconv"
	"time"
)

// Clock controls a page's emulated time — a Playwright-style fake clock that
// overrides Date, setTimeout/setInterval, performance.now and
// requestAnimationFrame so tests can freeze time and fast-forward timers
// deterministically. Obtain one with Page.Clock(); install with Clock.Install
// or the WithClock launch option, then drive it with FastForward/SetSystemTime.
type Clock struct{ page *Page }

// Clock returns a controller for the page's emulated clock.
func (p *Page) Clock() *Clock { return &Clock{page: p} }

// clockCall wraps body so the fake clock is installed (idempotently) before the
// body runs, allowing controller methods to work whether or not WithClock ran.
func clockCall(body string) string {
	return "() => { if (!window.__scoutClock) {" + clockScript + "} " + body + " }"
}

func msStr(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }

// Install installs the fake clock and sets the current time to at. After this,
// page code sees Date/timers driven by this controller. Safe to call repeatedly.
func (c *Clock) Install(at time.Time) error {
	if _, err := c.page.Eval(clockCall("window.__scoutClock.install(" + msStr(at) + ");")); err != nil {
		return fmt.Errorf("scout: clock install: %w", err)
	}
	return nil
}

// SetSystemTime jumps the emulated current time to t without firing timers.
func (c *Clock) SetSystemTime(t time.Time) error {
	if _, err := c.page.Eval(clockCall("window.__scoutClock.setSystemTime(" + msStr(t) + ");")); err != nil {
		return fmt.Errorf("scout: clock set system time: %w", err)
	}
	return nil
}

// SetFixedTime pins Date to t (Date.now() and new Date() report t).
func (c *Clock) SetFixedTime(t time.Time) error {
	if _, err := c.page.Eval(clockCall("window.__scoutClock.setFixedTime(" + msStr(t) + ");")); err != nil {
		return fmt.Errorf("scout: clock set fixed time: %w", err)
	}
	return nil
}

// FastForward advances the clock by d, firing every timer that comes due (in
// scheduled order). setInterval callbacks fire once per interval crossed.
func (c *Clock) FastForward(d time.Duration) error {
	ms := strconv.FormatInt(d.Milliseconds(), 10)
	if _, err := c.page.Eval(clockCall("window.__scoutClock.tick(" + ms + ");")); err != nil {
		return fmt.Errorf("scout: clock fast-forward: %w", err)
	}
	return nil
}

// RunFor is an alias for FastForward.
func (c *Clock) RunFor(d time.Duration) error { return c.FastForward(d) }

// Now returns the clock's current emulated time.
func (c *Clock) Now() (time.Time, error) {
	r, err := c.page.Eval(clockCall("return window.__scoutClock.now();"))
	if err != nil {
		return time.Time{}, fmt.Errorf("scout: clock now: %w", err)
	}
	return time.UnixMilli(int64(r.Float())), nil
}

// WithClock installs an emulated clock initialized to at before any page script
// runs, on every navigation. Drive it afterwards with Page.Clock().
func WithClock(at time.Time) Option {
	script := clockScript + ";window.__scoutClock.install(" + strconv.FormatInt(at.UnixMilli(), 10) + ");"
	return func(o *options) { o.injectScripts = append(o.injectScripts, script) }
}
