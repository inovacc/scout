package engine

import (
	"context"
	"encoding/json"
	"time"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

// workerSnapshotExpr evaluates to the main-thread navigator values a Web Worker must
// mirror to clear the hasInconsistentWorkerValues bot signal. The User-Agent is
// already unified across the main thread and workers by the process-global
// --user-agent switch (see resolveUserAgent); these are the fields Scout's stealth
// patches on the main thread only. Returned by value via Runtime.evaluate.
const workerSnapshotExpr = `({` +
	`hc:navigator.hardwareConcurrency,` +
	`dm:navigator.deviceMemory,` +
	`plat:navigator.platform,` +
	`wd:navigator.webdriver,` +
	`lang:navigator.language,` +
	`langs:navigator.languages` +
	`})`

// workerSnapshotTimeout bounds the main-thread read done when a worker attaches, so a
// page that is itself synchronously awaiting this worker cannot wedge the read.
const workerSnapshotTimeout = 3 * time.Second

// installWorkerStealth makes Web Workers spawned by the page present the same navigator
// identity as the stealth-patched main thread. Scout's evasions run on the main thread
// only, so a worker otherwise reports unpatched hardwareConcurrency, deviceMemory,
// languages and webdriver — the inconsistency deviceandbrowserinfo and CreepJS key on
// (hasInconsistentWorkerValues), even in headed+stealth.
//
// It auto-attaches to this page's worker targets paused-on-start, mirrors a live
// snapshot of the main thread's spoofed values into each worker, then resumes it (the
// resume is deferred so a paused worker is ALWAYS released). Each worker is handled on
// its own goroutine.
//
// OPT-IN via WithWorkerStealth(): pausing a worker to inject before it runs is the only
// mechanism that reliably reaches every worker type without breaking it, but a page
// that synchronously awaits its OWN worker inside a single blocking Eval can then stall
// (the awaited worker can't run until we resume it, and the resume can't complete until
// the caller's Eval frees the session). No real site or normal scraping does this, but
// to keep the default stealth path strictly hang-free this is not enabled by default.
func installWorkerStealth(p *rodPage) {
	if p == nil {
		return
	}

	pageSession := string(p.SessionID)

	if _, err := p.browser.Call(p.ctx, pageSession, "Target.setAutoAttach", map[string]any{
		"autoAttach":             true,
		"waitForDebuggerOnStart": true,
		"flatten":                true,
	}); err != nil {
		// Main-thread stealth still applies; worker mirroring is best-effort.
		return
	}

	go func() {
		defer func() { _ = recover() }()

		p.EachEvent(func(e *proto2.TargetAttachedToTarget) { //nolint:staticcheck // internalized rod API
			if e == nil || e.TargetInfo == nil {
				return
			}

			workerSession := string(e.SessionID)
			workerType := e.TargetInfo.Type

			// Handle each attached target on its own goroutine so the event-dispatch
			// loop keeps flowing; the resume is deferred so a paused target is ALWAYS
			// released even if injection fails or panics.
			go func() {
				defer func() { _ = recover() }()
				defer func() {
					_, _ = p.browser.Call(p.ctx, workerSession, "Runtime.runIfWaitingForDebugger", struct{}{})
				}()

				switch workerType {
				case "worker", "shared_worker", "service_worker":
				default:
					return // not a worker — the deferred resume still releases it
				}

				snap, ok := mainNavigatorSnapshot(p, pageSession)
				if !ok {
					return // could not read main values in time — resume uninjected
				}

				script := "(function(){var v=" + snap + ";" +
					"function d(k,val){try{Object.defineProperty(navigator,k,{get:function(){return val},configurable:true})}catch(e){}}" +
					"d('hardwareConcurrency',v.hc);d('deviceMemory',v.dm);d('platform',v.plat);d('webdriver',v.wd);d('language',v.lang);" +
					"try{Object.defineProperty(navigator,'languages',{get:function(){return v.langs},configurable:true})}catch(e){}" +
					"})()"

				_, _ = p.browser.Call(p.ctx, workerSession, "Runtime.evaluate", map[string]any{
					"expression":    script,
					"returnByValue": true,
				})
			}()
		})
	}()
}

// mainNavigatorSnapshot reads the page main thread's spoofed navigator values as a JSON
// object literal, bounded by workerSnapshotTimeout. Returns (json, true) on success.
func mainNavigatorSnapshot(p *rodPage, pageSession string) (string, bool) {
	ctx, cancel := context.WithTimeout(p.ctx, workerSnapshotTimeout)
	defer cancel()

	raw, err := p.browser.Call(ctx, pageSession, "Runtime.evaluate", map[string]any{
		"expression":    workerSnapshotExpr,
		"returnByValue": true,
	})
	if err != nil {
		return "", false
	}

	var resp struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *json.RawMessage `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", false
	}
	if resp.ExceptionDetails != nil || len(resp.Result.Value) == 0 {
		return "", false
	}

	return string(resp.Result.Value), true
}
