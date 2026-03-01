package launcher

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/inovacc/scout/pkg/rod/lib/launcher/flags"
)

func TestNewDefaults(t *testing.T) {
	l := New()

	if !l.Has(flags.Headless) {
		t.Fatal("expected headless enabled by default")
	}
	if l.Get(flags.Bin) == "" {
		// Bin defaults to empty string from defaults.Bin
	}
	if !l.Has(flags.UserDataDir) {
		t.Fatal("expected user-data-dir set by default")
	}
	if l.Has(flags.Flag("rod-leakless")) {
		t.Fatal("leakless flag should not be set")
	}
}

func TestSetGetDeleteHas(t *testing.T) {
	l := New()

	l.Set(flags.ProxyServer, "http://localhost:8080")
	if got := l.Get(flags.ProxyServer); got != "http://localhost:8080" {
		t.Fatalf("expected proxy, got %q", got)
	}
	if !l.Has(flags.ProxyServer) {
		t.Fatal("expected Has(ProxyServer) true")
	}

	l.Delete(flags.ProxyServer)
	if l.Has(flags.ProxyServer) {
		t.Fatal("expected Has(ProxyServer) false after delete")
	}
}

func TestAppend(t *testing.T) {
	l := New()
	l.Set(flags.Flag("disable-features"), "a")
	l.Append(flags.Flag("disable-features"), "b", "c")

	vals, ok := l.GetFlags(flags.Flag("disable-features"))
	if !ok || len(vals) != 3 {
		t.Fatalf("expected 3 values, got %v", vals)
	}
}

func TestHeadlessToggle(t *testing.T) {
	l := New()
	l.Headless(false)
	if l.Has(flags.Headless) {
		t.Fatal("headless should be disabled")
	}
	l.Headless(true)
	if !l.Has(flags.Headless) {
		t.Fatal("headless should be enabled")
	}
}

func TestHeadlessNew(t *testing.T) {
	l := New()
	l.HeadlessNew(true)
	if got := l.Get(flags.Headless); got != "new" {
		t.Fatalf("expected headless=new, got %q", got)
	}
	l.HeadlessNew(false)
	if l.Has(flags.Headless) {
		t.Fatal("headless should be removed")
	}
}

func TestNoSandbox(t *testing.T) {
	l := New()
	l.NoSandbox(true)
	if !l.Has(flags.NoSandbox) {
		t.Fatal("expected no-sandbox")
	}
	l.NoSandbox(false)
	if l.Has(flags.NoSandbox) {
		t.Fatal("expected no-sandbox removed")
	}
}

func TestProxy(t *testing.T) {
	l := New()
	l.Proxy("socks5://localhost:1080")
	if got := l.Get(flags.ProxyServer); got != "socks5://localhost:1080" {
		t.Fatalf("expected proxy, got %q", got)
	}
}

func TestUserDataDir(t *testing.T) {
	l := New()
	l.UserDataDir("/tmp/test")
	if got := l.Get(flags.UserDataDir); got != "/tmp/test" {
		t.Fatalf("expected /tmp/test, got %q", got)
	}
	l.UserDataDir("")
	if l.Has(flags.UserDataDir) {
		t.Fatal("expected user-data-dir removed")
	}
}

func TestProfileDir(t *testing.T) {
	l := New()
	l.ProfileDir("MyProfile")
	if got := l.Get(flags.ProfileDir); got != "MyProfile" {
		t.Fatalf("expected MyProfile, got %q", got)
	}
	l.ProfileDir("")
	if l.Has(flags.ProfileDir) {
		t.Fatal("expected profile-dir removed")
	}
}

func TestWindowSizeAndPosition(t *testing.T) {
	l := New()
	l.WindowSize(800, 600)
	if got := l.Get(flags.WindowSize); got != "800,600" {
		t.Fatalf("expected 800,600, got %q", got)
	}
	l.WindowPosition(100, 200)
	if got := l.Get(flags.WindowPosition); got != "100,200" {
		t.Fatalf("expected 100,200, got %q", got)
	}
}

func TestRemoteDebuggingPort(t *testing.T) {
	l := New()
	l.RemoteDebuggingPort(9222)
	if got := l.Get(flags.RemoteDebuggingPort); got != "9222" {
		t.Fatalf("expected 9222, got %q", got)
	}
}

func TestEnv(t *testing.T) {
	l := New()
	l.Env("TZ=UTC", "FOO=bar")
	vals, ok := l.GetFlags(flags.Env)
	if !ok || len(vals) != 2 {
		t.Fatalf("expected 2 env values, got %v", vals)
	}
}

func TestDevtools(t *testing.T) {
	l := New()
	l.Devtools(true)
	if !l.Has(flags.Flag("auto-open-devtools-for-tabs")) {
		t.Fatal("expected devtools flag")
	}
	l.Devtools(false)
	if l.Has(flags.Flag("auto-open-devtools-for-tabs")) {
		t.Fatal("expected devtools flag removed")
	}
}

func TestFormatArgs(t *testing.T) {
	l := New()
	l.Set(flags.Flag("disable-gpu"))
	args := l.FormatArgs()

	found := false
	for _, a := range args {
		if a == "--disable-gpu" {
			found = true
		}
		// rod- prefixed flags should be excluded
		if strings.HasPrefix(a, "--rod-") {
			t.Fatalf("rod- flag leaked into args: %s", a)
		}
	}
	if !found {
		t.Fatal("expected --disable-gpu in formatted args")
	}
}

func TestFormatArgsExcludesRodFlags(t *testing.T) {
	l := New()
	args := l.FormatArgs()
	for _, a := range args {
		if strings.HasPrefix(a, "--rod-") {
			t.Fatalf("rod- flag should not appear in CLI args: %s", a)
		}
	}
}

func TestStartURL(t *testing.T) {
	l := New()
	l.StartURL("http://example.com")
	// StartURL uses the empty-string flag key (Arguments)
	vals, ok := l.GetFlags(flags.Arguments)
	if !ok || len(vals) != 1 || vals[0] != "http://example.com" {
		t.Fatalf("expected start URL in arguments, got %v", vals)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := New().Context(ctx)
	cancel()

	// After cancel, the launcher's context should be done.
	select {
	case <-l.ctx.Done():
		// expected
	default:
		t.Fatal("expected context to be cancelled")
	}
}

func TestLaunchAlreadyLaunched(t *testing.T) {
	l := New()
	// Simulate already launched.
	atomic.StoreInt32(&l.isLaunched, 1)

	_, err := l.Launch()
	if err != ErrAlreadyLaunched {
		t.Fatalf("expected ErrAlreadyLaunched, got %v", err)
	}
}

func TestPID(t *testing.T) {
	l := New()
	if l.PID() != 0 {
		t.Fatal("expected PID 0 before launch")
	}
}

func TestNewUserMode(t *testing.T) {
	l := NewUserMode()
	if l.Get(flags.RemoteDebuggingPort) != "37712" {
		t.Fatal("expected port 37712 for user mode")
	}
	// User mode should not have headless
	if l.Has(flags.Headless) {
		t.Fatal("user mode should not have headless")
	}
}

func TestNewAppMode(t *testing.T) {
	l := NewAppMode("http://example.com")
	if !l.Has(flags.App) {
		t.Fatal("expected app flag set")
	}
	if l.Has(flags.Headless) {
		t.Fatal("app mode should not be headless")
	}
	if l.Has(flags.Flag("no-startup-window")) {
		t.Fatal("app mode should not have no-startup-window")
	}
}

func TestAlwaysOpenPDFExternally(t *testing.T) {
	l := New()
	l.AlwaysOpenPDFExternally()
	pref := l.Get(flags.Preferences)
	if !strings.Contains(pref, "always_open_pdf_externally") {
		t.Fatal("expected PDF pref set")
	}
}

func TestKillWithZeroPID(t *testing.T) {
	l := New()
	// Should not panic with PID 0.
	l.Kill()
}

func TestWorkingDir(t *testing.T) {
	l := New()
	l.WorkingDir("/tmp")
	if got := l.Get(flags.WorkingDir); got != "/tmp" {
		t.Fatalf("expected /tmp, got %q", got)
	}
}
