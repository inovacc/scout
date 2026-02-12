// Package slack provides a scraper for extracting data from Slack workspaces.
//
// It uses a hybrid approach: browser automation for authentication (handling SSO,
// 2FA, and interactive login) combined with Slack's internal web API for fast,
// reliable data extraction.
//
// # Quick Start with Token
//
//	s := slack.New(
//	    slack.WithWorkspace("myteam.slack.com"),
//	    slack.WithToken("xoxc-..."),
//	    slack.WithDCookie("xoxd-..."),
//	)
//
//	if err := s.Authenticate(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	channels, err := s.ListChannels(ctx)
//
// # Browser Login
//
//	s := slack.New(
//	    slack.WithWorkspace("myteam.slack.com"),
//	    slack.WithHeadless(false), // interactive login
//	)
//
//	// Opens browser for manual login, extracts token automatically
//	if err := s.Authenticate(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	messages, err := s.GetMessages(ctx, "C01ABC123")
package slack
