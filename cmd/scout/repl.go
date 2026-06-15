package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/inovacc/scout/internal/interaction"
	"github.com/inovacc/scout/internal/redact"
	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(replCmd)
}

var replCmd = &cobra.Command{
	Use:   "repl [url]",
	Short: "Interactive local browser shell (no daemon required)",
	Long: `Launch a browser and interact with it via commands.
Commands: navigate, eval, click, type, extract, screenshot, markdown, html,
  cookies, url, title, wait, back, forward, reload, tabs, tab, newtab,
  health, help, exit`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := baseOpts(cmd)

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: repl: %w", err)
		}

		defer func() { _ = b.Close() }()

		var page *scout.Page
		if len(args) > 0 && args[0] != "" {
			// Adapter bookkeeping: create the first page, then navigate via the verb.
			page, err = b.NewPage("")
			if err != nil {
				return fmt.Errorf("scout: repl: %w", err)
			}

			if _, err := tools.Navigate(cmd.Context(), page, tools.NavigateInput{URL: args[0]}); err != nil {
				return fmt.Errorf("scout: repl: navigate: %w", err)
			}
		}

		interaction.Init("repl")
		defer func() { _ = interaction.Close("ok") }()

		out := cmd.OutOrStdout()
		scanner := bufio.NewScanner(os.Stdin)

		for {
			prompt := "> "

			if page != nil {
				if u, err := page.URL(); err == nil {
					if parsed, err := url.Parse(u); err == nil {
						prompt = fmt.Sprintf("[%s%s] > ", parsed.Host, parsed.Path)
					}
				}
			}

			_, _ = fmt.Fprint(out, prompt)

			if !scanner.Scan() {
				break
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			parts := strings.SplitN(line, " ", 3)
			c := parts[0]

			switch c {
			case "exit", "quit", "help":
				// meta commands — not captured
			default:
				args := redact.Args(parts[1:])
				for i := range args {
					args[i] = redact.URL(args[i]) // mask tokens in positional URLs
				}

				interaction.Emit(interaction.Event{
					Kind:   "browser_action",
					Source: "repl",
					Name:   c,
					Input:  map[string]any{"args": args},
				})
			}

			switch c {
			case "exit", "quit":
				_, _ = fmt.Fprintln(out, "Bye!")
				return nil

			case "help":
				printREPLHelp(out)

			case "navigate", "go", "nav":
				if len(parts) < 2 {
					_, _ = fmt.Fprintln(out, "usage: navigate <url>")
					continue
				}

				// Adapter bookkeeping: open a fresh page to navigate (preserves the
				// "each navigate is a new tab" REPL UX). The navigation itself
				// routes through the tools.Navigate verb.
				newPage, err := b.NewPage("")
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				res, err := tools.Navigate(cmd.Context(), newPage, tools.NavigateInput{URL: parts[1]})
				if err != nil {
					_ = newPage.Close()
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				if page != nil {
					_ = page.Close()
				}

				page = newPage

				_, _ = fmt.Fprintf(out, "Page: %s\n", res.Title)

			case "eval":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if len(parts) < 2 {
					_, _ = fmt.Fprintln(out, "usage: eval <js expression>")
					continue
				}

				expr := strings.Join(parts[1:], " ")

				res, err := tools.Eval(cmd.Context(), page, tools.EvalInput{Expression: expr})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				_, _ = fmt.Fprintln(out, res.Result)

			case "click":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if len(parts) < 2 {
					_, _ = fmt.Fprintln(out, "usage: click <selector>")
					continue
				}

				if _, err := tools.Click(cmd.Context(), page, tools.ClickInput{Selector: parts[1]}); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, "clicked")
				}

			case "type":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if len(parts) < 3 {
					_, _ = fmt.Fprintln(out, "usage: type <selector> <text>")
					continue
				}

				if _, err := tools.Type(cmd.Context(), page, tools.TypeInput{Selector: parts[1], Text: parts[2]}); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, "typed")
				}

			case "extract":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if len(parts) < 2 {
					_, _ = fmt.Fprintln(out, "usage: extract <selector>")
					continue
				}

				res, err := tools.Extract(cmd.Context(), page, tools.ExtractInput{Selector: parts[1]})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, res.Text)
				}

			case "screenshot":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				filename := "screenshot.png"
				if len(parts) >= 2 {
					filename = parts[1]
				}

				res, err := tools.Screenshot(cmd.Context(), page, tools.ScreenshotInput{})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				if err := os.WriteFile(filename, res.Data, 0o644); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintf(out, "Saved: %s\n", filename)
				}

			case "markdown", "md":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				res, err := tools.Markdown(cmd.Context(), page, tools.MarkdownInput{})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, res.Markdown)
				}

			case "html":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				res, err := tools.HTML(cmd.Context(), page, tools.HTMLInput{})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, res.HTML)
				}

			case "cookies":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				res, err := tools.Cookies(cmd.Context(), page, tools.CookiesInput{})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")

				if err := enc.Encode(res.Cookies); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				}

			case "url":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				res, err := tools.URL(cmd.Context(), page, tools.URLInput{})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, res.URL)
				}

			case "title":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				res, err := tools.Title(cmd.Context(), page, tools.TitleInput{})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, res.Title)
				}

			case "wait":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if len(parts) < 2 {
					if _, err := tools.Wait(cmd.Context(), page, tools.WaitInput{}); err != nil {
						_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					} else {
						_, _ = fmt.Fprintln(out, "page loaded")
					}
				} else {
					// Wait for element to exist (Wait blocks until found or timeout).
					if _, err := tools.Wait(cmd.Context(), page, tools.WaitInput{Selector: parts[1]}); err != nil {
						_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					} else {
						_, _ = fmt.Fprintf(out, "found: %s\n", parts[1])
					}
				}

			case "back":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if _, err := tools.Back(cmd.Context(), page, tools.BackInput{}); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				}

			case "forward":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if _, err := tools.Forward(cmd.Context(), page, tools.ForwardInput{}); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				}

			case "reload":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if _, err := tools.Reload(cmd.Context(), page, tools.ReloadInput{}); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, "reloaded")
				}

			case "tabs":
				res, err := tools.Tabs(cmd.Context(), b, tools.TabsInput{})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				// Adapter bookkeeping: highlight the current tab by URL.
				var currentURL string
				if page != nil {
					currentURL, _ = page.URL()
				}

				for _, tab := range res.Tabs {
					marker := "  "
					if currentURL != "" && tab.URL == currentURL {
						marker = "* "
					}

					_, _ = fmt.Fprintf(out, "%s[%d] %s - %s\n", marker, tab.Index, truncate(tab.Title, 40), truncate(tab.URL, 60))
				}

			case "tab":
				if len(parts) < 2 {
					_, _ = fmt.Fprintln(out, "usage: tab <index>")
					continue
				}

				// Adapter bookkeeping: switching the "current tab" requires the
				// concrete *scout.Page handle, which is local REPL state.
				pages, err := b.Pages()
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				var idx int
				if _, err := fmt.Sscanf(parts[1], "%d", &idx); err != nil || idx < 0 || idx >= len(pages) {
					_, _ = fmt.Fprintf(out, "invalid tab index: %s\n", parts[1])
					continue
				}

				page = pages[idx]

				res, err := tools.Title(cmd.Context(), page, tools.TitleInput{})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				_, _ = fmt.Fprintf(out, "Switched to: %s\n", res.Title)

			case "newtab":
				u := ""
				if len(parts) >= 2 {
					u = parts[1]
				}

				if _, err := tools.NewTab(cmd.Context(), b, tools.NewTabInput{URL: u}); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				// Adapter bookkeeping: the new tab is the last opened page; track
				// it as the current page for subsequent commands.
				pages, err := b.Pages()
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				if len(pages) > 0 {
					page = pages[len(pages)-1]
				}

				_, _ = fmt.Fprintln(out, "new tab opened")

			case "health":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				urlRes, err := tools.URL(cmd.Context(), page, tools.URLInput{})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				report, err := tools.TestSite(cmd.Context(), b, tools.TestSiteInput{URL: urlRes.URL, Depth: 1, Concurrency: 1})
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				_, _ = fmt.Fprintf(out, "Pages: %d  Duration: %s  Issues: %d\n", report.Pages, report.Duration, len(report.Issues))

				for _, issue := range report.Issues {
					_, _ = fmt.Fprintf(out, "  [%s] %s: %s\n", issue.Severity, issue.Source, issue.Message)
				}

			default:
				_, _ = fmt.Fprintf(out, "unknown command: %s (type 'help' for commands)\n", c)
			}
		}

		return nil
	},
}

func printREPLHelp(out io.Writer) {
	_, _ = fmt.Fprintln(out, `Commands:
  navigate/go/nav <url>  Navigate to URL
  eval <js>              Evaluate JavaScript
  click <selector>       Click an element
  type <sel> <text>      Type text into element
  extract <selector>     Extract text from element
  screenshot [file]      Take screenshot (default: screenshot.png)
  markdown/md            Get page as markdown
  html                   Get page HTML
  cookies                Show page cookies
  url                    Show current URL
  title                  Show page title
  wait [selector]        Wait for page load or element
  back                   Navigate back
  forward                Navigate forward
  reload                 Reload page
  tabs                   List open tabs
  tab <index>            Switch to tab
  newtab [url]           Open new tab
  health                 Run health check on current page
  help                   Show this help
  exit/quit              Exit REPL`)
}
