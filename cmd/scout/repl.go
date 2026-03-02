package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
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
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:maintidx
		opts := baseOpts(cmd)

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: repl: %w", err)
		}

		defer func() { _ = b.Close() }()

		var page *scout.Page
		if len(args) > 0 && args[0] != "" {
			page, err = b.NewPage(args[0])
			if err != nil {
				return fmt.Errorf("scout: repl: navigate: %w", err)
			}

			_ = page.WaitLoad()
		}

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

				newPage, err := b.NewPage(parts[1])
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				_ = newPage.WaitLoad()

				if page != nil {
					_ = page.Close()
				}

				page = newPage

				title, _ := page.Title()
				_, _ = fmt.Fprintf(out, "Page: %s\n", title)

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

				result, err := page.Eval(expr)
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				_, _ = fmt.Fprintln(out, result)

			case "click":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if len(parts) < 2 {
					_, _ = fmt.Fprintln(out, "usage: click <selector>")
					continue
				}

				el, err := page.Element(parts[1])
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				if err := el.Click(); err != nil {
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

				el, err := page.Element(parts[1])
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				if err := el.Input(parts[2]); err != nil {
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

				text, err := page.ExtractText(parts[1])
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, text)
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

				data, err := page.Screenshot()
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				if err := os.WriteFile(filename, data, 0o644); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintf(out, "Saved: %s\n", filename)
				}

			case "markdown", "md":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				md, err := page.Markdown()
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, md)
				}

			case "html":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				html, err := page.HTML()
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, html)
				}

			case "cookies":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				cookies, err := page.GetCookies()
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				_ = enc.Encode(cookies)

			case "url":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				u, err := page.URL()
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, u)
				}

			case "title":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				title, err := page.Title()
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, title)
				}

			case "wait":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if len(parts) < 2 {
					_ = page.WaitLoad()
					_, _ = fmt.Fprintln(out, "page loaded")
				} else {
					// Wait for element to exist (Element blocks until found or timeout).
					if _, err := page.Element(parts[1]); err != nil {
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

				if err := page.NavigateBack(); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				}

			case "forward":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if err := page.NavigateForward(); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				}

			case "reload":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				if err := page.Reload(); err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				} else {
					_, _ = fmt.Fprintln(out, "reloaded")
				}

			case "tabs":
				pages, err := b.Pages()
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				for i, p := range pages {
					u, _ := p.URL()
					t, _ := p.Title()

					marker := "  "
					if page != nil {
						if pu, _ := page.URL(); pu == u {
							marker = "* "
						}
					}

					_, _ = fmt.Fprintf(out, "%s[%d] %s - %s\n", marker, i, truncate(t, 40), truncate(u, 60))
				}

			case "tab":
				if len(parts) < 2 {
					_, _ = fmt.Fprintln(out, "usage: tab <index>")
					continue
				}

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

				title, _ := page.Title()
				_, _ = fmt.Fprintf(out, "Switched to: %s\n", title)

			case "newtab":
				u := ""
				if len(parts) >= 2 {
					u = parts[1]
				}

				newPage, err := b.NewPage(u)
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}

				if u != "" {
					_ = newPage.WaitLoad()
				}

				page = newPage

				_, _ = fmt.Fprintln(out, "new tab opened")

			case "health":
				if page == nil {
					_, _ = fmt.Fprintln(out, "no page open")
					continue
				}

				u, _ := page.URL()
				report, err := b.HealthCheck(u, scout.WithHealthDepth(1), scout.WithHealthConcurrency(1))
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
