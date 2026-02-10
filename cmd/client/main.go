package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	addr := flag.String("addr", "localhost:50051", "gRPC server address")
	url := flag.String("url", "https://example.com", "initial URL")
	flag.Parse()

	conn, err := grpc.NewClient(*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64*1024*1024),
			grpc.MaxCallSendMsgSize(64*1024*1024),
		),
	)
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewScoutServiceClient(conn)
	ctx := context.Background()

	// Create session with recording
	fmt.Println("Creating browser session...")
	sess, err := client.CreateSession(ctx, &pb.CreateSessionRequest{
		Headless:    true,
		Stealth:     true,
		InitialUrl:  *url,
		Record:      true,
		CaptureBody: true,
	})
	if err != nil {
		log.Fatalf("create session failed: %v", err)
	}
	fmt.Printf("Session: %s\n", sess.SessionId)
	fmt.Printf("Page: %s - %s\n\n", sess.Title, sess.Url)

	// Start event stream in background
	stream, err := client.StreamEvents(ctx, &pb.SessionRequest{SessionId: sess.SessionId})
	if err != nil {
		log.Fatalf("stream events failed: %v", err)
	}

	go func() {
		for {
			ev, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("stream error: %v", err)
				return
			}
			printEvent(ev)
		}
	}()

	// Interactive command loop
	fmt.Println("Commands: nav <url> | click <sel> | type <sel> <text> | key <key>")
	fmt.Println("          text <sel> | title | url | eval <js> | shot [full]")
	fmt.Println("          har <file> | quit")
	fmt.Println(strings.Repeat("-", 70))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n scout> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		cmd := parts[0]

		switch cmd {
		case "nav", "navigate":
			if len(parts) < 2 {
				fmt.Println("usage: nav <url>")
				continue
			}
			resp, err := client.Navigate(ctx, &pb.NavigateRequest{
				SessionId:  sess.SessionId,
				Url:        parts[1],
				WaitStable: true,
			})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			fmt.Printf("Page: %s - %s\n", resp.Title, resp.Url)

		case "click":
			if len(parts) < 2 {
				fmt.Println("usage: click <selector>")
				continue
			}
			_, err := client.Click(ctx, &pb.ElementRequest{
				SessionId: sess.SessionId,
				Selector:  parts[1],
			})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			fmt.Println("clicked")

		case "type":
			if len(parts) < 3 {
				fmt.Println("usage: type <selector> <text>")
				continue
			}
			_, err := client.Type(ctx, &pb.TypeRequest{
				SessionId:  sess.SessionId,
				Selector:   parts[1],
				Text:       parts[2],
				ClearFirst: true,
			})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			fmt.Println("typed")

		case "key":
			if len(parts) < 2 {
				fmt.Println("usage: key <Enter|Tab|Escape|...>")
				continue
			}
			_, err := client.PressKey(ctx, &pb.KeyRequest{
				SessionId: sess.SessionId,
				Key:       parts[1],
			})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			fmt.Println("key pressed")

		case "text":
			if len(parts) < 2 {
				fmt.Println("usage: text <selector>")
				continue
			}
			resp, err := client.GetText(ctx, &pb.ElementRequest{
				SessionId: sess.SessionId,
				Selector:  parts[1],
			})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			fmt.Printf("text: %s\n", resp.Text)

		case "title":
			resp, err := client.GetTitle(ctx, &pb.SessionRequest{SessionId: sess.SessionId})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			fmt.Printf("title: %s\n", resp.Text)

		case "url":
			resp, err := client.GetURL(ctx, &pb.SessionRequest{SessionId: sess.SessionId})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			fmt.Printf("url: %s\n", resp.Text)

		case "eval":
			if len(parts) < 2 {
				fmt.Println("usage: eval <javascript>")
				continue
			}
			script := strings.Join(parts[1:], " ")
			resp, err := client.Eval(ctx, &pb.EvalRequest{
				SessionId: sess.SessionId,
				Script:    script,
			})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			fmt.Printf("result: %s\n", resp.Result)

		case "shot", "screenshot":
			fullPage := len(parts) > 1 && parts[1] == "full"
			resp, err := client.Screenshot(ctx, &pb.ScreenshotRequest{
				SessionId: sess.SessionId,
				FullPage:  fullPage,
			})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			filename := fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
			if err := os.WriteFile(filename, resp.Data, 0o644); err != nil {
				fmt.Printf("ERROR writing file: %v\n", err)
				continue
			}
			fmt.Printf("saved to %s (%d bytes)\n", filename, len(resp.Data))

		case "har":
			resp, err := client.ExportHAR(ctx, &pb.SessionRequest{SessionId: sess.SessionId})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}
			filename := "capture.har"
			if len(parts) > 1 {
				filename = parts[1]
			}
			if err := os.WriteFile(filename, resp.Data, 0o644); err != nil {
				fmt.Printf("ERROR writing file: %v\n", err)
				continue
			}
			fmt.Printf("HAR exported: %s (%d entries)\n", filename, resp.EntryCount)

		case "quit", "exit":
			fmt.Println("Exporting HAR before exit...")
			resp, _ := client.ExportHAR(ctx, &pb.SessionRequest{SessionId: sess.SessionId})
			if resp != nil && resp.EntryCount > 0 {
				filename := fmt.Sprintf("forensic_%d.har", time.Now().Unix())
				if err := os.WriteFile(filename, resp.Data, 0o644); err != nil {
					fmt.Printf("ERROR writing file: %v\n", err)
				} else {
					fmt.Printf("Final HAR: %s (%d entries)\n", filename, resp.EntryCount)
				}
			}
			_, _ = client.DestroySession(ctx, &pb.SessionRequest{SessionId: sess.SessionId})
			fmt.Println("Session destroyed. Bye!")
			return

		default:
			fmt.Printf("unknown command: %s\n", cmd)
		}
	}
}

func printEvent(ev *pb.BrowserEvent) {
	ts := time.UnixMilli(ev.Timestamp).Format("15:04:05.000")

	switch e := ev.Event.(type) {
	case *pb.BrowserEvent_RequestSent:
		fmt.Printf("\n  [%s] -> %s %s\n", ts, e.RequestSent.Method, truncate(e.RequestSent.Url, 80))

	case *pb.BrowserEvent_ResponseReceived:
		fmt.Printf("  [%s] <- %d %s (%.0fms) %s\n",
			ts,
			e.ResponseReceived.Status,
			truncate(e.ResponseReceived.Url, 60),
			e.ResponseReceived.TimeMs,
			e.ResponseReceived.MimeType,
		)

	case *pb.BrowserEvent_Console:
		fmt.Printf("  [%s] console.%s: %s\n", ts, e.Console.Level, truncate(e.Console.Message, 80))

	case *pb.BrowserEvent_PageEvent:
		fmt.Printf("  [%s] page.%s: %s\n", ts, e.PageEvent.Type, e.PageEvent.Url)

	case *pb.BrowserEvent_Error:
		fmt.Printf("  [%s] ERROR: %s (source: %s)\n", ts, e.Error.Message, e.Error.Source)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
