package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// This example demonstrates using the bidirectional Interactive stream
// to automate a login flow while capturing all network traffic for forensics.
func main() {
	addr := flag.String("addr", "localhost:50051", "gRPC server address")
	target := flag.String("target", "https://httpbin.org/forms/post", "target URL")
	flag.Parse()

	conn, err := grpc.NewClient(*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewScoutServiceClient(conn)
	ctx := context.Background()

	// Create session with forensic recording
	sess, err := client.CreateSession(ctx, &pb.CreateSessionRequest{
		Headless:    true,
		Stealth:     true,
		Record:      true,
		CaptureBody: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_, _ = client.DestroySession(ctx, &pb.SessionRequest{SessionId: sess.SessionId})
	}()

	fmt.Printf("Session: %s\n\n", sess.SessionId)

	// Open bidirectional stream
	stream, err := client.Interactive(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Receive events in background
	eventCount := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			ev, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}

			switch e := ev.Event.(type) {
			case *pb.BrowserEvent_RequestSent:
				eventCount++
				fmt.Printf("  -> %s %s\n", e.RequestSent.Method, e.RequestSent.Url)
			case *pb.BrowserEvent_ResponseReceived:
				fmt.Printf("  <- %d %s\n", e.ResponseReceived.Status, e.ResponseReceived.MimeType)
			case *pb.BrowserEvent_Error:
				fmt.Printf("  ERROR: %s\n", e.Error.Message)
			}
		}
	}()

	// Automated workflow: navigate, fill form, submit

	fmt.Printf("Step 1: Navigate to %s\n", *target)
	_ = stream.Send(&pb.Command{
		SessionId: sess.SessionId,
		RequestId: "1",
		Action:    &pb.Command_Navigate{Navigate: &pb.NavigateAction{Url: *target}},
	})
	time.Sleep(2 * time.Second)

	fmt.Println("Step 2: Fill form fields")
	_ = stream.Send(&pb.Command{
		SessionId: sess.SessionId,
		RequestId: "2",
		Action: &pb.Command_Type{Type: &pb.TypeAction{
			Selector: "input[name='custname']",
			Text:     "Test User",
		}},
	})
	time.Sleep(500 * time.Millisecond)

	_ = stream.Send(&pb.Command{
		SessionId: sess.SessionId,
		RequestId: "3",
		Action: &pb.Command_Type{Type: &pb.TypeAction{
			Selector: "input[name='custtel']",
			Text:     "+5511999999999",
		}},
	})
	time.Sleep(500 * time.Millisecond)

	fmt.Println("Step 3: Click submit")
	_ = stream.Send(&pb.Command{
		SessionId: sess.SessionId,
		RequestId: "4",
		Action: &pb.Command_Click{Click: &pb.ClickAction{
			Selector: "button[type='submit']",
		}},
	})
	time.Sleep(2 * time.Second)

	fmt.Println("Step 4: Take screenshot")
	_ = stream.Send(&pb.Command{
		SessionId: sess.SessionId,
		RequestId: "5",
		Action:    &pb.Command_Screenshot{Screenshot: &pb.ScreenshotAction{FullPage: true}},
	})
	time.Sleep(1 * time.Second)

	// Close the interactive stream
	_ = stream.CloseSend()
	<-done

	// Export forensic data
	fmt.Printf("\nCaptured %d network events\n", eventCount)

	har, err := client.ExportHAR(ctx, &pb.SessionRequest{SessionId: sess.SessionId})
	if err != nil {
		log.Fatal(err)
	}

	filename := fmt.Sprintf("forensic_%d.har", time.Now().Unix())
	if err := os.WriteFile(filename, har.Data, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("HAR saved: %s (%d entries)\n", filename, har.EntryCount)
	fmt.Println("   Open in Chrome DevTools -> Network tab -> Import HAR")
}
