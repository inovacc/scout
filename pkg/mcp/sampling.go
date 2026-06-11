/*
Copyright (c) 2026 Security Research

Package-level sampling singleton. The production MCPClient calls back to the
host (Claude Code) via MCP sampling/createMessage reverse-RPC. Activation is
daemon-only (D-03/D-04): cmd/mcp.go calls SetSession after Server.Connect.

All failure modes (D-06) WARN-log and return an error so adapters can swallow
and degrade gracefully (NilMCPClient semantics).
*/

package mcp

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	samplingMaxTokens int64         = 2000             // D-10
	samplingTimeout   time.Duration = 30 * time.Second // D-10
)

var (
	sessionMu sync.RWMutex
	session   *gomcp.ServerSession
	logger    *slog.Logger

	// ErrNoSession is returned by Sample when SetSession has not been called
	// or was cleared. Callers can use errors.Is to distinguish this from
	// transport failures.
	ErrNoSession = errors.New("pkg/mcp: sampling client not wired (daemon not running as `unravel mcp serve`)")
	// ErrNonText is returned by Sample when the host responds with non-text
	// content (e.g. an image or audio block).
	ErrNonText = errors.New("pkg/mcp: sampling host returned non-text content")

	// unexported aliases kept for white-box tests in this package.
	errNoSession = ErrNoSession
	errNonText   = ErrNonText
)

// SetSession is called from cmd/mcp.go after gomcp.Server.Connect (D-12).
// Passing nil clears the session (used by tests for cleanup).
func SetSession(ss *gomcp.ServerSession, log *slog.Logger) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	session = ss
	if log == nil {
		log = slog.Default()
	}
	logger = log
}

// Sample is the exported round-trip used by all adapters. It calls the
// connected MCP host via sampling/createMessage reverse-RPC and returns the
// text response as raw bytes.
//
// Uses the default token budget (samplingMaxTokens). For larger responses
// — typical of structured data extraction — use SampleWithMaxTokens.
//
// D-06: every error path WARN-logs and returns the error so the adapter
// can swallow it and return a zero-value to the caller.
func Sample(ctx context.Context, prompt string) ([]byte, error) {
	return SampleWithMaxTokens(ctx, prompt, samplingMaxTokens)
}

// SampleWithMaxTokens is Sample with a caller-specified output token budget.
// Use this when the default 2000 cap is too small (e.g. extraction returning
// large structured payloads). Same warn-log + error semantics as Sample.
//
// maxTokens <= 0 is treated as the default samplingMaxTokens.
func SampleWithMaxTokens(ctx context.Context, prompt string, maxTokens int64) ([]byte, error) {
	if maxTokens <= 0 {
		maxTokens = samplingMaxTokens
	}

	sessionMu.RLock()
	ss, log := session, logger
	sessionMu.RUnlock()

	if log == nil {
		log = slog.Default()
	}

	if ss == nil {
		log.Warn("sampling/createMessage: no session", "error", errNoSession)
		return nil, errNoSession
	}

	cctx, cancel := context.WithTimeout(ctx, samplingTimeout)
	defer cancel()

	res, err := ss.CreateMessage(cctx, &gomcp.CreateMessageParams{
		MaxTokens: maxTokens,
		Messages: []*gomcp.SamplingMessage{{
			Role:    "user",
			Content: &gomcp.TextContent{Text: prompt},
		}},
	})
	if err != nil {
		log.Warn("sampling/createMessage failed", "error", err)
		return nil, err
	}

	txt, ok := res.Content.(*gomcp.TextContent)
	if !ok || txt == nil {
		log.Warn("sampling/createMessage: non-text content", "model", res.Model)
		return nil, errNonText
	}

	return []byte(txt.Text), nil
}

// HasSession reports whether a non-nil session is currently wired.
// Internal/mcp adapter resolvers use this to decide between a live
// adapter and the domain's NilMCPClient fallback.
func HasSession() bool {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	return session != nil
}
