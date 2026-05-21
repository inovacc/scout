package engine

import (
	"strings"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

// ConsoleMessage is a structured browser console event surfaced via
// Page.OnConsole. Level matches CDP's RuntimeConsoleAPICalledType
// (log, info, warning, error, debug, etc.).
type ConsoleMessage struct {
	Level string
	Text  string
	URL   string
	Line  int
}

// OnConsole subscribes to Runtime.consoleAPICalled events and invokes cb
// for each. The returned cancel function detaches the subscription.
// Spawns a goroutine for the event pump; safe to call once per Page.
func (p *Page) OnConsole(cb func(ConsoleMessage)) func() {
	if p == nil || p.page == nil || cb == nil {
		return func() {}
	}

	stop := make(chan struct{})

	go func() {
		wait := p.page.EachEvent(func(e *proto2.RuntimeConsoleAPICalled) {
			var parts []string
			for _, arg := range e.Args {
				parts = append(parts, arg.Value.Str())
			}
			msg := ConsoleMessage{
				Level: string(e.Type),
				Text:  strings.Join(parts, " "),
			}
			if e.StackTrace != nil && len(e.StackTrace.CallFrames) > 0 {
				msg.URL = e.StackTrace.CallFrames[0].URL
				msg.Line = e.StackTrace.CallFrames[0].LineNumber
			}
			cb(msg)
		})
		// EachEvent's wait blocks until any callback returns true; our
		// callbacks return nothing, so this only unblocks when the
		// underlying client context cancels. Wait in a select so the
		// stop signal also unblocks us.
		done := make(chan struct{})
		go func() { wait(); close(done) }()
		select {
		case <-stop:
		case <-done:
		}
	}()

	return func() { close(stop) }
}
