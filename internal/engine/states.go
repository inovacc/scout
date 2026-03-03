package engine

import (
	"reflect"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

type stateKey struct {
	browserContextID proto2.BrowserBrowserContextID
	sessionID        proto2.TargetSessionID
	methodName       string
}

func (b *rodBrowser) key(sessionID proto2.TargetSessionID, methodName string) stateKey {
	return stateKey{
		browserContextID: b.BrowserContextID,
		sessionID:        sessionID,
		methodName:       methodName,
	}
}

func (b *rodBrowser) set(sessionID proto2.TargetSessionID, methodName string, params any) {
	b.states.Store(b.key(sessionID, methodName), params)

	key := ""

	switch methodName {
	case (proto2.EmulationClearDeviceMetricsOverride{}).ProtoReq():
		key = (proto2.EmulationSetDeviceMetricsOverride{}).ProtoReq()
	case (proto2.EmulationClearGeolocationOverride{}).ProtoReq():
		key = (proto2.EmulationSetGeolocationOverride{}).ProtoReq()
	default:
		domain, name := proto2.ParseMethodName(methodName)
		if name == "disable" {
			key = domain + ".enable"
		}
	}

	if key != "" {
		b.states.Delete(b.key(sessionID, key))
	}
}

// LoadState into the method, sessionID can be empty.
func (b *rodBrowser) LoadState(sessionID proto2.TargetSessionID, method proto2.Request) (has bool) {
	data, has := b.states.Load(b.key(sessionID, method.ProtoReq()))
	if has {
		reflect.Indirect(reflect.ValueOf(method)).Set(
			reflect.Indirect(reflect.ValueOf(data)),
		)
	}

	return
}

// RemoveState a state.
func (b *rodBrowser) RemoveState(key any) {
	b.states.Delete(key)
}

// EnableDomain and returns a restore function to restore previous state.
func (b *rodBrowser) EnableDomain(sessionID proto2.TargetSessionID, req proto2.Request) (restore func()) {
	_, enabled := b.states.Load(b.key(sessionID, req.ProtoReq()))

	if !enabled {
		_, _ = b.Call(b.ctx, string(sessionID), req.ProtoReq(), req)
	}

	return func() {
		if !enabled {
			domain, _ := proto2.ParseMethodName(req.ProtoReq())
			_, _ = b.Call(b.ctx, string(sessionID), domain+".disable", nil)
		}
	}
}

// DisableDomain and returns a restore function to restore previous state.
func (b *rodBrowser) DisableDomain(sessionID proto2.TargetSessionID, req proto2.Request) (restore func()) {
	_, enabled := b.states.Load(b.key(sessionID, req.ProtoReq()))
	domain, _ := proto2.ParseMethodName(req.ProtoReq())

	if enabled {
		_, _ = b.Call(b.ctx, string(sessionID), domain+".disable", nil)
	}

	return func() {
		if enabled {
			_, _ = b.Call(b.ctx, string(sessionID), req.ProtoReq(), req)
		}
	}
}

func (b *rodBrowser) cachePage(page *rodPage) {
	b.states.Store(page.TargetID, page)
}

func (b *rodBrowser) loadCachedPage(id proto2.TargetTargetID) *rodPage {
	if cache, ok := b.states.Load(id); ok {
		return cache.(*rodPage) //nolint: forcetypeassert
	}

	return nil
}

// LoadState into the method.
func (p *rodPage) LoadState(method proto2.Request) (has bool) {
	return p.browser.LoadState(p.SessionID, method)
}

// EnableDomain and returns a restore function to restore previous state.
func (p *rodPage) EnableDomain(method proto2.Request) (restore func()) {
	return p.browser.Context(p.ctx).EnableDomain(p.SessionID, method)
}

// DisableDomain and returns a restore function to restore previous state.
func (p *rodPage) DisableDomain(method proto2.Request) (restore func()) {
	return p.browser.Context(p.ctx).DisableDomain(p.SessionID, method)
}

func (p *rodPage) cleanupStates() {
	p.browser.RemoveState(p.TargetID)
}
