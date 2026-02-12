package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(clientCmd)

	clientCmd.Flags().String("url", "https://example.com", "initial URL")
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Interactive gRPC client REPL",
	RunE: func(cmd *cobra.Command, _ []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		url, _ := cmd.Flags().GetString("url")

		if err := ensureDaemon(addr); err != nil {
			return err
		}

		client, conn, err := getClient(addr)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		ctx := context.Background()

		// Create session with recording
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Creating browser session...")
		sess, err := client.CreateSession(ctx, &pb.CreateSessionRequest{
			Headless:    true,
			Stealth:     true,
			InitialUrl:  url,
			Record:      true,
			CaptureBody: true,
		})
		if err != nil {
			return fmt.Errorf("scout: create session: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Session: %s\n", sess.GetSessionId())
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Page: %s - %s\n\n", sess.GetTitle(), sess.GetUrl())

		// Start event stream in background
		stream, err := client.StreamEvents(ctx, &pb.SessionRequest{SessionId: sess.GetSessionId()})
		if err != nil {
			return fmt.Errorf("scout: stream events: %w", err)
		}

		go func() {
			for {
				ev, err := stream.Recv()
				if err == io.EOF || err != nil {
					return
				}
				printEvent(cmd, ev)
			}
		}()

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Commands: nav <url> | click <sel> | type <sel> <text> | key <key>")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "          text <sel> | title | url | eval <js> | shot [full]")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "          har <file> | quit")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 70))

		scanner := bufio.NewScanner(os.Stdin)
		for {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), "\n scout> ")
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
			case "nav", "navigate":
				if len(parts) < 2 {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "usage: nav <url>")
					continue
				}
				resp, err := client.Navigate(ctx, &pb.NavigateRequest{
					SessionId:  sess.GetSessionId(),
					Url:        parts[1],
					WaitStable: true,
				})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Page: %s - %s\n", resp.GetTitle(), resp.GetUrl())

			case "click":
				if len(parts) < 2 {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "usage: click <selector>")
					continue
				}
				_, err := client.Click(ctx, &pb.ElementRequest{
					SessionId: sess.GetSessionId(),
					Selector:  parts[1],
				})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "clicked")

			case "type":
				if len(parts) < 3 {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "usage: type <selector> <text>")
					continue
				}
				_, err := client.Type(ctx, &pb.TypeRequest{
					SessionId:  sess.GetSessionId(),
					Selector:   parts[1],
					Text:       parts[2],
					ClearFirst: true,
				})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "typed")

			case "key":
				if len(parts) < 2 {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "usage: key <Enter|Tab|Escape|...>")
					continue
				}
				_, err := client.PressKey(ctx, &pb.KeyRequest{
					SessionId: sess.GetSessionId(),
					Key:       parts[1],
				})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "key pressed")

			case "text":
				if len(parts) < 2 {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "usage: text <selector>")
					continue
				}
				resp, err := client.GetText(ctx, &pb.ElementRequest{
					SessionId: sess.GetSessionId(),
					Selector:  parts[1],
				})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "text: %s\n", resp.GetText())

			case "title":
				resp, err := client.GetTitle(ctx, &pb.SessionRequest{SessionId: sess.GetSessionId()})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "title: %s\n", resp.GetText())

			case "url":
				resp, err := client.GetURL(ctx, &pb.SessionRequest{SessionId: sess.GetSessionId()})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "url: %s\n", resp.GetText())

			case "eval":
				if len(parts) < 2 {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "usage: eval <javascript>")
					continue
				}
				script := strings.Join(parts[1:], " ")
				resp, err := client.Eval(ctx, &pb.EvalRequest{
					SessionId: sess.GetSessionId(),
					Script:    script,
				})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "result: %s\n", resp.GetResult())

			case "shot", "screenshot":
				fullPage := len(parts) > 1 && parts[1] == "full"
				resp, err := client.Screenshot(ctx, &pb.ScreenshotRequest{
					SessionId: sess.GetSessionId(),
					FullPage:  fullPage,
				})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				filename := fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
				if err := os.WriteFile(filename, resp.GetData(), 0o644); err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR writing file: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved to %s (%d bytes)\n", filename, len(resp.GetData()))

			case "har":
				resp, err := client.ExportHAR(ctx, &pb.SessionRequest{SessionId: sess.GetSessionId()})
				if err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %v\n", err)
					continue
				}
				filename := "capture.har"
				if len(parts) > 1 {
					filename = parts[1]
				}
				if err := os.WriteFile(filename, resp.GetData(), 0o644); err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR writing file: %v\n", err)
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "HAR exported: %s (%d entries)\n", filename, resp.GetEntryCount())

			case "quit", "exit":
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Exporting HAR before exit...")
				resp, _ := client.ExportHAR(ctx, &pb.SessionRequest{SessionId: sess.GetSessionId()})
				if resp != nil && resp.GetEntryCount() > 0 {
					filename := fmt.Sprintf("forensic_%d.har", time.Now().Unix())
					if err := os.WriteFile(filename, resp.GetData(), 0o644); err != nil {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR writing file: %v\n", err)
					} else {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Final HAR: %s (%d entries)\n", filename, resp.GetEntryCount())
					}
				}
				_, _ = client.DestroySession(ctx, &pb.SessionRequest{SessionId: sess.GetSessionId()})
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Session destroyed. Bye!")
				return nil

			default:
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "unknown command: %s\n", c)
			}
		}

		return nil
	},
}

func printEvent(cmd *cobra.Command, ev *pb.BrowserEvent) {
	w := cmd.OutOrStdout()
	ts := time.UnixMilli(ev.GetTimestamp()).Format("15:04:05.000")

	switch e := ev.GetEvent().(type) {
	case *pb.BrowserEvent_RequestSent:
		_, _ = fmt.Fprintf(w, "\n  [%s] -> %s %s\n", ts, e.RequestSent.GetMethod(), truncate(e.RequestSent.GetUrl(), 80))
	case *pb.BrowserEvent_ResponseReceived:
		_, _ = fmt.Fprintf(w, "  [%s] <- %d %s (%.0fms) %s\n",
			ts, e.ResponseReceived.GetStatus(), truncate(e.ResponseReceived.GetUrl(), 60),
			e.ResponseReceived.GetTimeMs(), e.ResponseReceived.GetMimeType())
	case *pb.BrowserEvent_Console:
		_, _ = fmt.Fprintf(w, "  [%s] console.%s: %s\n", ts, e.Console.GetLevel(), truncate(e.Console.GetMessage(), 80))
	case *pb.BrowserEvent_PageEvent:
		_, _ = fmt.Fprintf(w, "  [%s] page.%s: %s\n", ts, e.PageEvent.GetType(), e.PageEvent.GetUrl())
	case *pb.BrowserEvent_Error:
		_, _ = fmt.Fprintf(w, "  [%s] ERROR: %s (source: %s)\n", ts, e.Error.GetMessage(), e.Error.GetSource())
	}
}
