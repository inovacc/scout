package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/pkg/discovery"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

type instance struct {
	label    string
	addr     string
	client   pb.ScoutServiceClient
	conn     *grpc.ClientConn
	sessID   string
}

func init() {
	rootCmd.AddCommand(clientCmd)

	clientCmd.Flags().String("url", "https://example.com", "initial URL")
	clientCmd.Flags().Bool("discover", false, "auto-discover local instances via mDNS")
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Interactive gRPC client REPL (supports multiple targets)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		url, _ := cmd.Flags().GetString("url")
		discoverFlag, _ := cmd.Flags().GetBool("discover")
		targets, _ := cmd.Flags().GetStringSlice("target")
		addr, _ := cmd.Flags().GetString("addr")

		// Build target list
		if len(targets) == 0 && !discoverFlag {
			targets = []string{addr}
		}

		// Discover targets via mDNS
		if discoverFlag {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Discovering scout instances (3s)...")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			peers, err := discovery.Discover(ctx)
			if err != nil {
				cancel()
				return fmt.Errorf("scout: discover: %w", err)
			}
			for peer := range peers {
				if len(peer.Addrs) > 0 {
					t := fmt.Sprintf("%s:%d", peer.Addrs[0], peer.Port)
					targets = append(targets, t)
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Found: %s (%s)\n", t, peer.DeviceID[:15]+"...")
				}
			}
			cancel()
		}

		if len(targets) == 0 {
			return fmt.Errorf("scout: no targets specified (use --target or --discover)")
		}

		// Connect to all targets
		var instances []*instance
		for i, t := range targets {
			if err := ensureDaemon(t); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", t, err)
				continue
			}
			insecureFlag, _ := cmd.Flags().GetBool("insecure")
			var (
				client pb.ScoutServiceClient
				conn   *grpc.ClientConn
				err    error
			)
			if insecureFlag {
				client, conn, err = getClient(t)
			} else {
				client, conn, err = getClientTLS(t)
				if err != nil {
					// Fall back to insecure if TLS fails (e.g. no identity)
					client, conn, err = getClient(t)
				}
			}
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", t, err)
				continue
			}
			label := fmt.Sprintf("node%d", i)
			if len(targets) == 1 {
				label = ""
			}
			instances = append(instances, &instance{label: label, addr: t, client: client, conn: conn})
		}

		if len(instances) == 0 {
			return fmt.Errorf("scout: no reachable targets")
		}

		defer func() {
			for _, inst := range instances {
				_ = inst.conn.Close()
			}
		}()

		ctx := context.Background()

		headless, _ := cmd.Flags().GetBool("headless")

		// Create sessions on all instances
		for _, inst := range instances {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Creating session on %s...\n", inst.addr)
			sess, err := inst.client.CreateSession(ctx, &pb.CreateSessionRequest{
				Headless:    headless,
				Stealth:     true,
				InitialUrl:  url,
				Record:      true,
				CaptureBody: true,
			})
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  ERROR: %v\n", err)
				continue
			}
			inst.sessID = sess.GetSessionId()
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Session: %s  Page: %s - %s\n",
				sess.GetSessionId(), sess.GetTitle(), sess.GetUrl())
		}

		// Start event streams
		for _, inst := range instances {
			if inst.sessID == "" {
				continue
			}
			go streamInstanceEvents(cmd, ctx, inst)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nCommands: nav <url> | click <sel> | type <sel> <text> | key <key>")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "          text <sel> | title | url | eval <js> | shot [full]")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "          har <file> | quit")
		if len(instances) > 1 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Prefix with @nodeN to target specific instance (e.g. @node0 nav https://...)")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Without prefix, commands are broadcast to all instances.")
		}
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

			// Check for @nodeN prefix
			var targetInstances []*instance
			if strings.HasPrefix(line, "@") && len(instances) > 1 {
				space := strings.IndexByte(line, ' ')
				if space > 0 {
					prefix := line[1:space]
					line = strings.TrimSpace(line[space+1:])
					for _, inst := range instances {
						if inst.label == prefix {
							targetInstances = []*instance{inst}
							break
						}
					}
					if len(targetInstances) == 0 {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "unknown target: %s\n", prefix)
						continue
					}
				}
			}
			if len(targetInstances) == 0 {
				targetInstances = instances
			}

			parts := strings.SplitN(line, " ", 3)
			c := parts[0]

			if c == "quit" || c == "exit" {
				for _, inst := range instances {
					if inst.sessID == "" {
						continue
					}
					resp, _ := inst.client.ExportHAR(ctx, &pb.SessionRequest{SessionId: inst.sessID})
					if resp != nil && resp.GetEntryCount() > 0 {
						filename := fmt.Sprintf("forensic_%s_%d.har", inst.label, time.Now().Unix())
						if len(instances) == 1 {
							filename = fmt.Sprintf("forensic_%d.har", time.Now().Unix())
						}
						if err := os.WriteFile(filename, resp.GetData(), 0o644); err != nil {
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ERROR writing %s: %v\n", filename, err)
						} else {
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "HAR: %s (%d entries)\n", filename, resp.GetEntryCount())
						}
					}
					_, _ = inst.client.DestroySession(ctx, &pb.SessionRequest{SessionId: inst.sessID})
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Sessions destroyed. Bye!")
				return nil
			}

			// Execute command on target instances
			var wg sync.WaitGroup
			for _, inst := range targetInstances {
				if inst.sessID == "" {
					continue
				}
				wg.Add(1)
				go func(inst *instance) {
					defer wg.Done()
					executeREPLCommand(cmd, ctx, inst, c, parts)
				}(inst)
			}
			wg.Wait()
		}

		return nil
	},
}

func executeREPLCommand(cmd *cobra.Command, ctx context.Context, inst *instance, c string, parts []string) {
	prefix := ""
	if inst.label != "" {
		prefix = fmt.Sprintf("[%s] ", inst.label)
	}

	switch c {
	case "nav", "navigate":
		if len(parts) < 2 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%susage: nav <url>\n", prefix)
			return
		}
		resp, err := inst.client.Navigate(ctx, &pb.NavigateRequest{
			SessionId:  inst.sessID,
			Url:        parts[1],
			WaitStable: true,
		})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sPage: %s - %s\n", prefix, resp.GetTitle(), resp.GetUrl())

	case "click":
		if len(parts) < 2 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%susage: click <selector>\n", prefix)
			return
		}
		_, err := inst.client.Click(ctx, &pb.ElementRequest{
			SessionId: inst.sessID,
			Selector:  parts[1],
		})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sclicked\n", prefix)

	case "type":
		if len(parts) < 3 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%susage: type <selector> <text>\n", prefix)
			return
		}
		_, err := inst.client.Type(ctx, &pb.TypeRequest{
			SessionId:  inst.sessID,
			Selector:   parts[1],
			Text:       parts[2],
			ClearFirst: true,
		})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%styped\n", prefix)

	case "key":
		if len(parts) < 2 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%susage: key <Enter|Tab|Escape|...>\n", prefix)
			return
		}
		_, err := inst.client.PressKey(ctx, &pb.KeyRequest{
			SessionId: inst.sessID,
			Key:       parts[1],
		})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%skey pressed\n", prefix)

	case "text":
		if len(parts) < 2 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%susage: text <selector>\n", prefix)
			return
		}
		resp, err := inst.client.GetText(ctx, &pb.ElementRequest{
			SessionId: inst.sessID,
			Selector:  parts[1],
		})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%stext: %s\n", prefix, resp.GetText())

	case "title":
		resp, err := inst.client.GetTitle(ctx, &pb.SessionRequest{SessionId: inst.sessID})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%stitle: %s\n", prefix, resp.GetText())

	case "url":
		resp, err := inst.client.GetURL(ctx, &pb.SessionRequest{SessionId: inst.sessID})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%surl: %s\n", prefix, resp.GetText())

	case "eval":
		if len(parts) < 2 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%susage: eval <javascript>\n", prefix)
			return
		}
		script := strings.Join(parts[1:], " ")
		resp, err := inst.client.Eval(ctx, &pb.EvalRequest{
			SessionId: inst.sessID,
			Script:    script,
		})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sresult: %s\n", prefix, resp.GetResult())

	case "shot", "screenshot":
		fullPage := len(parts) > 1 && parts[1] == "full"
		resp, err := inst.client.Screenshot(ctx, &pb.ScreenshotRequest{
			SessionId: inst.sessID,
			FullPage:  fullPage,
		})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		filename := fmt.Sprintf("screenshot_%s_%d.png", inst.label, time.Now().Unix())
		if inst.label == "" {
			filename = fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
		}
		if err := os.WriteFile(filename, resp.GetData(), 0o644); err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR writing file: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%ssaved to %s (%d bytes)\n", prefix, filename, len(resp.GetData()))

	case "har":
		resp, err := inst.client.ExportHAR(ctx, &pb.SessionRequest{SessionId: inst.sessID})
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR: %v\n", prefix, err)
			return
		}
		filename := "capture.har"
		if len(parts) > 1 {
			filename = parts[1]
		}
		if inst.label != "" {
			filename = inst.label + "_" + filename
		}
		if err := os.WriteFile(filename, resp.GetData(), 0o644); err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sERROR writing file: %v\n", prefix, err)
			return
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sHAR exported: %s (%d entries)\n", prefix, filename, resp.GetEntryCount())

	default:
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sunknown command: %s\n", prefix, c)
	}
}

func streamInstanceEvents(cmd *cobra.Command, ctx context.Context, inst *instance) {
	stream, err := inst.client.StreamEvents(ctx, &pb.SessionRequest{SessionId: inst.sessID})
	if err != nil {
		return
	}

	for {
		ev, err := stream.Recv()
		if err == io.EOF || err != nil {
			return
		}
		printInstanceEvent(cmd, inst, ev)
	}
}

func printInstanceEvent(cmd *cobra.Command, inst *instance, ev *pb.BrowserEvent) {
	w := cmd.OutOrStdout()
	ts := time.UnixMilli(ev.GetTimestamp()).Format("15:04:05.000")
	prefix := ""
	if inst.label != "" {
		prefix = fmt.Sprintf("[%s]", inst.label)
	}

	switch e := ev.GetEvent().(type) {
	case *pb.BrowserEvent_RequestSent:
		_, _ = fmt.Fprintf(w, "\n  %s[%s] -> %s %s\n", prefix, ts, e.RequestSent.GetMethod(), truncate(e.RequestSent.GetUrl(), 80))
	case *pb.BrowserEvent_ResponseReceived:
		_, _ = fmt.Fprintf(w, "  %s[%s] <- %d %s (%.0fms) %s\n",
			prefix, ts, e.ResponseReceived.GetStatus(), truncate(e.ResponseReceived.GetUrl(), 60),
			e.ResponseReceived.GetTimeMs(), e.ResponseReceived.GetMimeType())
	case *pb.BrowserEvent_Console:
		_, _ = fmt.Fprintf(w, "  %s[%s] console.%s: %s\n", prefix, ts, e.Console.GetLevel(), truncate(e.Console.GetMessage(), 80))
	case *pb.BrowserEvent_PageEvent:
		_, _ = fmt.Fprintf(w, "  %s[%s] page.%s: %s\n", prefix, ts, e.PageEvent.GetType(), e.PageEvent.GetUrl())
	case *pb.BrowserEvent_Error:
		_, _ = fmt.Fprintf(w, "  %s[%s] ERROR: %s (source: %s)\n", prefix, ts, e.Error.GetMessage(), e.Error.GetSource())
	}
}
