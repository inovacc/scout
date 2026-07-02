package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/inovacc/scout/pkg/scout/scraper"
)

// ModeProxy implements scraper.Mode by forwarding calls to a plugin subprocess.
type ModeProxy struct {
	entry    ModeEntry
	manifest *Manifest
	manager  *Manager
}

func (m *ModeProxy) Name() string                       { return m.entry.Name }
func (m *ModeProxy) Description() string                { return m.entry.Description }
func (m *ModeProxy) AuthProvider() scraper.AuthProvider { return nil }

// Scrape launches the plugin (if needed), sends a scrape request, and streams results.
func (m *ModeProxy) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	client, err := m.manager.getClient(m.manifest) //nolint:contextcheck // getClient manages process lifecycle, not request context
	if err != nil {
		return nil, fmt.Errorf("plugin: start %s: %w", m.manifest.Name, err)
	}

	params := map[string]any{
		"mode": m.entry.Name,
		"options": map[string]any{
			"headless": opts.Headless,
			"stealth":  opts.Stealth,
			"limit":    opts.Limit,
			"targets":  opts.Targets,
		},
	}

	ch := make(chan scraper.Result, 32)

	// Two producers write to ch: the batch goroutine (Call return value) and the
	// notification drainer (streamed "result" messages that arrive during Call).
	// To make close-of-ch race-free, the batch goroutine is the SOLE closer: it
	// signals the drainer to stop (stopStream), waits until the drainer has
	// stopped sending (streamDone), and only then closes ch. Previously the
	// batch goroutine closed ch while the drainer was still sending, panicking
	// the whole process with "send on closed channel".
	stopStream := make(chan struct{})
	streamDone := make(chan struct{})

	// Drain notifications for streamed results.
	go func() {
		defer close(streamDone)

		for {
			select {
			case <-ctx.Done():
				return
			case <-stopStream:
				return
			case notif, ok := <-client.Notifications():
				if !ok {
					return
				}

				switch notif.Method {
				case "result":
					var r scraper.Result
					if err := json.Unmarshal(notif.Params, &r); err == nil {
						select {
						case ch <- r:
						case <-ctx.Done():
							return
						case <-stopStream:
							return
						}
					}
				case "log":
					var logMsg struct {
						Level   string `json:"level"`
						Message string `json:"message"`
					}

					if err := json.Unmarshal(notif.Params, &logMsg); err == nil {
						m.manager.logger.Log(ctx, parseLevel(logMsg.Level), logMsg.Message, "plugin", m.manifest.Name)
					}
				}
			}
		}
	}()

	// Batch goroutine + sole closer of ch.
	go func() {
		defer func() {
			close(stopStream) // tell the drainer to stop sending
			<-streamDone      // wait until it has stopped
			close(ch)         // now the only sender remaining is us, and we are done
		}()

		result, err := client.Call(ctx, "scrape", params)
		if err != nil {
			m.manager.logger.Error("plugin: scrape call failed", "plugin", m.manifest.Name, "error", err)
			return
		}

		// Result may be a single batch or we may receive streamed notifications.
		// Try to decode as array of results first.
		var results []scraper.Result
		if err := json.Unmarshal(result, &results); err == nil {
			for _, r := range results {
				select {
				case ch <- r:
				case <-ctx.Done():
					return
				}
			}

			return
		}

		// Single result.
		var single scraper.Result
		if err := json.Unmarshal(result, &single); err == nil {
			select {
			case ch <- single:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
