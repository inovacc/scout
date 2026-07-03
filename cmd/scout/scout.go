package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/gops/agent"
	"github.com/inovacc/scout/internal/engine/session"
	"github.com/inovacc/scout/internal/flags"
	"github.com/inovacc/scout/internal/interaction"
	"github.com/inovacc/scout/internal/logger"
	"github.com/inovacc/scout/internal/redact"
	"github.com/inovacc/scout/internal/tracing"
	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/plugin"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scout",
	Short: "Headless browser automation, scraping, and forensic capture",
	Long: `Scout is a CLI for headless browser automation, web scraping, search, crawling,
and forensic capture. It wraps go-rod and exposes all features as commands.

Sessions are per-command (no daemon): each invocation opens and cleanly closes
its own browser. Multi-step automation uses ` + "`scout repl`" + `, the stdio MCP
server (` + "`scout mcp`" + `), or strategy/runbook files.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := flags.ExportFlagsToEnv(); err != nil {
			// Non-fatal, but do NOT return early: the old `return nil` silently
			// skipped logger + interaction-capture init below — disabling exactly
			// the diagnostics you want when the cache dir is broken. Warn and go on.
			slog.Warn("scout: export flags to env failed; continuing", "err", err)
		}

		if flags.ShouldIgnoreCommand(cmd.Name()) {
			return nil
		}

		log := logger.Init(cmd.Name())
		if log.IsActive() {
			stdout, stderr := log.StartExecution(cmd.Name(), args, cmd.OutOrStdout(), cmd.ErrOrStderr())
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
		}

		// Past the ShouldIgnoreCommand early-return above, so capture starts here
		// unconditionally for the (non-ignored) command.
		interaction.Init("cli")
		interaction.Emit(interaction.Event{
			Kind:   "cli",
			Source: "cli",
			Name:   cmd.Name(),
			Input:  map[string]any{"args": redact.Args(args)},
		})

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().String("session", "", "session ID to use")
	rootCmd.PersistentFlags().StringP("output", "o", "", "output file path")
	rootCmd.PersistentFlags().String("format", "text", "output format (text, json)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().Bool("headless", true, "run browser in headless mode")
	rootCmd.PersistentFlags().String("browser", "chrome", "browser type: chrome, chromium, brave, edge")
	rootCmd.PersistentFlags().Bool("maximized", false, "start browser window maximized")
	rootCmd.PersistentFlags().Bool("devtools", false, "open Chrome DevTools automatically")
	rootCmd.PersistentFlags().Bool("stealth", false, "enable anti-bot-detection stealth mode")
	rootCmd.PersistentFlags().Bool("worker-stealth", false, "extend stealth to Web Workers (opt-in; may stall a page that awaits its own worker)")
	rootCmd.PersistentFlags().String("electron-app", "", "path to Electron app directory or binary")
	rootCmd.PersistentFlags().String("electron-version", "", "Electron version to download (e.g. v33.2.0)")
	rootCmd.PersistentFlags().String("electron-cdp", "", "CDP endpoint of running Electron app")
	rootCmd.PersistentFlags().Duration("idle-timeout", 5*time.Minute, "auto-shutdown after inactivity (0 to disable)")
	rootCmd.PersistentFlags().Bool("system-browser", false, "allow system-installed browsers instead of cache-only")
	rootCmd.PersistentFlags().String("user-data-dir", "", "path to browser user data directory (e.g. Chrome profile)")
	rootCmd.PersistentFlags().String("profile-directory", "Default", "Chrome profile directory name")
}

func Execute() {
	registerPluginCommands()

	exitCode := func() (code int) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("scout: cli: recovered panic", "panic", r, "stack", string(debug.Stack()))
				_, _ = fmt.Fprintf(os.Stderr, "scout: internal error: %v\n", r)
				code = 1
			}
		}()

		err := rootCmd.Execute()

		log := logger.Get()
		if log != nil && log.IsActive() {
			log.EndExecution(err)
			_ = log.Close()
		}

		// A command that failed before PersistentPreRunE ran (arg-validation
		// error or unknown command) never started a capture. Record it now so
		// these high-signal "friction" invocations are not lost.
		if err != nil && interaction.Default() == nil {
			if name := firstPositional(os.Args[1:]); !flags.ShouldIgnoreCommand(name) {
				interaction.Init("cli")
				interaction.Emit(interaction.Event{
					Kind:   "cli",
					Source: "cli",
					Name:   name,
					Input:  map[string]any{"args": redact.Args(os.Args[1:])},
					Error:  err.Error(),
				})
			}
		}

		status := "ok"
		if err != nil {
			status = "error"
		}
		_ = interaction.Close(status)

		if err != nil {
			var ece *plugin.ExitCodeError
			if !errors.As(err, &ece) {
				// rootCmd has SilenceErrors=true so cobra never prints. Print
				// here so failures aren't silently swallowed. Plugin commands
				// that exited nonzero already emitted their own output, so we
				// pass their code through without an extra "scout:" line.
				_, _ = fmt.Fprintf(os.Stderr, "scout: %v\n", err)
			}

			return exitCodeFor(err)
		}
		return 0
	}()

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// exitCodeFor maps a command error to a process exit code. A plugin command that
// exited nonzero carries its own code (via plugin.ExitCodeError); every other
// failure is a generic error (1). This is the single exit chokepoint — no code
// outside Execute() should call os.Exit for a command failure.
func exitCodeFor(err error) int {
	var ece *plugin.ExitCodeError
	if errors.As(err, &ece) && ece.Code != 0 {
		return ece.Code
	}

	return 1
}

// firstPositional returns the first non-flag argument (the attempted
// subcommand), or "" if none.
func firstPositional(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}

	return ""
}

// registerPluginCommands discovers plugins and registers any cli_command capabilities
// as Cobra commands on rootCmd. This runs before Execute so plugin commands appear in --help.
func registerPluginCommands() {
	mgr := initPluginManager()
	if mgr == nil {
		return
	}

	// Set up browser provisioner so plugins with requires_browser can get a CDP endpoint.
	plugin.SetBrowserProvisioner(provisionBrowserForPlugin)

	replaced := mgr.RegisterCLICommands(rootCmd)
	_ = replaced // logged by manager
}

// provisionBrowserForPlugin launches a browser using standard CLI flags and returns
// the CDP endpoint for the plugin to connect to.
func provisionBrowserForPlugin(cmd *cobra.Command) (*plugin.BrowserContext, error) {
	opts := baseOpts(cmd)

	b, err := scout.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("scout: provision browser: %w", err)
	}

	sessionID := b.SessionID()

	return &plugin.BrowserContext{
		CDPEndpoint: b.CDPURL(),
		SessionDir:  scout.SessionDataDir(sessionID),
		SessionID:   sessionID,
	}, nil
}

func main() {
	maybeRunCaptureHost() // native-messaging host mode: returns immediately for normal CLI use

	// The gops agent exposes an unauthenticated diagnostic endpoint on loopback,
	// reachable by any local user on a multi-user host. Cross-process discovery
	// (goprocess.Find) does not require it, so allow disabling via SCOUT_GOPS=0.
	if v := os.Getenv("SCOUT_GOPS"); v != "0" && v != "false" && v != "FALSE" {
		if err := agent.Listen(agent.Options{ShutdownCleanup: true}); err != nil {
			slog.Warn("scout: gops agent", "error", err)
		}

		defer agent.Close()
	}

	// Clean leftover sessions from previous runs (dead processes, orphaned
	// dirs, and legacy JSON-format scout.pid files from before the binary
	// cutover).
	//
	// Run OFF the critical path: a slow WMI/CIM service or an AV/indexer-wedged
	// session dir must never delay the user's command. The background retrier
	// and reaper watchdog below own ongoing cleanup, and ReapOnce is documented
	// idempotent + concurrency-safe, so a concurrent first sweep is safe.
	go func() { _, _ = session.CleanStaleSessions() }()

	// Background retrier for stale dirs that resisted the bounded
	// startup cleanup (Windows AV / Search Indexer / OneDrive holding
	// Chrome SQLite + LevelDB files). Runs for the process lifetime.
	session.StartCleanupRetrier(nil)

	// Start the single process-wide reaper watchdog. Idempotent — safe even
	// if a browser is created before this point (EnsureReaperWatchdog uses
	// sync.Once so the goroutine starts exactly once per process).
	scout.EnsureReaperWatchdog()

	shutdown, err := tracing.Init(context.Background(), tracing.Config{ServiceName: "scout"})
	if err != nil {
		slog.Warn("scout: tracing", "error", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	// Best-effort SIGINT/SIGTERM handler (session-hardening "best-effort"
	// tier): on Ctrl-C / kill, close every live browser so chrome children
	// and scout.lock files are not leaked, then exit 130. NOT the
	// OS-guaranteed tier (Job Object / CTRL_CLOSE are out of scope).
	_ = installSignalCleanup(closeAllLiveCleanup)

	Execute()
}
