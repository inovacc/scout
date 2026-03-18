// scout-comm is a Scout plugin providing communication platform scraper modes.
// It wraps the built-in Slack, Discord, Teams, and Reddit scraper modes as a
// standalone plugin binary for independent distribution.
//
// Install: scout plugin install ./plugins/scout-comm
// Or build: go build -o scout-comm ./plugins/scout-comm
package main

import (
	"context"
	"log"
	"time"

	"github.com/inovacc/scout/pkg/scout/plugin/sdk"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"

	// Register all communication modes.
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/discord"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/reddit"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/slack"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/teams"
)

func main() {
	srv := sdk.NewServer()

	// Register each mode as an SDK mode handler.
	for _, name := range []string{"slack", "discord", "teams", "reddit"} {
		mode, err := scraper.GetMode(name)
		if err != nil {
			log.Fatalf("mode %s not found: %v", name, err)
		}

		srv.RegisterMode(name, &modeAdapter{mode: mode})
	}

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

// modeAdapter wraps a scraper.Mode as an sdk.ModeHandler.
type modeAdapter struct {
	mode scraper.Mode
}

func (a *modeAdapter) Scrape(_ context.Context, params sdk.ScrapeParams) ([]sdk.Result, error) {
	opts := scraper.DefaultScrapeOptions()

	if targets, ok := params.Options["targets"].([]any); ok {
		for _, t := range targets {
			if s, ok := t.(string); ok {
				opts.Targets = append(opts.Targets, s)
			}
		}
	}

	if limit, ok := params.Options["limit"].(float64); ok {
		opts.Limit = int(limit)
	}

	if timeout, ok := params.Options["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			opts.Timeout = d
		}
	}

	if headless, ok := params.Options["headless"].(bool); ok {
		opts.Headless = headless
	}

	// Load session if provided.
	var session scraper.SessionData

	if sessionJSON, ok := params.Options["session"].(map[string]any); ok {
		s := &auth.Session{
			Provider: a.mode.Name(),
		}

		if url, ok := sessionJSON["url"].(string); ok {
			s.URL = url
		}

		session = s
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	ch, err := a.mode.Scrape(ctx, session, opts)
	if err != nil {
		return nil, err
	}

	var results []sdk.Result

	for r := range ch {
		results = append(results, sdk.Result{
			Type:      string(r.Type),
			Source:    r.Source,
			ID:        r.ID,
			Timestamp: r.Timestamp.Format(time.RFC3339),
			Author:    r.Author,
			Content:   r.Content,
			URL:       r.URL,
			Metadata:  r.Metadata,
		})
	}

	return results, nil
}
