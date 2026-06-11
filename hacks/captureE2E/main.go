// Command captureE2E is a standalone, fully-automated end-to-end driver for the
// "Scout Capture" Phase 2 MV3 session-capture extension. It exercises the real
// native-messaging round-trip on this machine using a SYNTHETIC local login —
// no real site, no mocking, no skipped steps.
//
// It mirrors docs/superpowers/specs/2026-06-10-scout-capture-phase2-e2e-checklist.md.
//
// All state is isolated under a fresh temp SCOUT_HOME so the user's real
// ~/.scout / vault / sessions are never touched. The only global side effect is
// the native-messaging host registration (HKCU registry + manifest under
// %APPDATA%\Scout), which is always removed by the mandatory cleanup uninstall.
//
// Run: go run ./hacks/captureE2E
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine/lib/cdp"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/vault"
)

const (
	passphrase  = "testpass"
	nativeHost  = "com.inovacc.scout.capture"
	stepTimeout = 90 * time.Second
)

// step records one acceptance step's outcome.
type step struct {
	name string
	pass bool
	note string
}

type runner struct {
	scoutExe string
	home     string
	steps    []*step
}

func (r *runner) record(name string, pass bool, format string, args ...any) *step {
	s := &step{name: name, pass: pass, note: fmt.Sprintf(format, args...)}
	r.steps = append(r.steps, s)
	status := "PASS"
	if !pass {
		status = "FAIL"
	}
	fmt.Printf("[%s] %-42s %s\n", status, name, s.note)
	return s
}

// scoutCmd runs scout.exe with the isolated SCOUT_HOME + passphrase env. stdinLine
// is fed to stdin (so the non-TTY passphrase reader gets it).
func (r *runner) scoutCmd(stdinLine string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stepTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.scoutExe, args...)
	cmd.Env = append(os.Environ(),
		"SCOUT_HOME="+r.home,
		"SCOUT_VAULT_PASSPHRASE="+passphrase,
		"SCOUT_PASSPHRASE="+passphrase, // helpers.go readPassphrase honors this too
	)
	if stdinLine != "" {
		cmd.Stdin = strings.NewReader(stdinLine)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func logf(format string, args ...any) {
	fmt.Printf("[%s] "+format+"\n", append([]any{time.Now().Format("15:04:05")}, args...)...)
}

func main() {
	os.Exit(run())
}

func run() int {
	exe := scoutExePath()
	home, err := os.MkdirTemp("", "scout-capture-e2e-*")
	if err != nil {
		fmt.Println("FATAL: mktemp:", err)
		return 1
	}
	r := &runner{scoutExe: exe, home: home}

	// CRITICAL: the in-process scout.New browser AND the native-messaging host
	// child (spawned by Chrome, which inherits this process's env) must both see
	// the isolated SCOUT_HOME. Set it on the driver process itself.
	_ = os.Setenv("SCOUT_HOME", home)
	_ = os.Setenv("SCOUT_VAULT_PASSPHRASE", passphrase)
	_ = os.Setenv("SCOUT_PASSPHRASE", passphrase)

	fmt.Println("=== Scout Capture Phase 2 — automated E2E ===")
	fmt.Println("scout.exe :", exe)
	fmt.Println("SCOUT_HOME:", home)
	fmt.Println()

	// Pre-seed the isolated browser cache from the default location so we reuse a
	// cached Chrome-for-Testing instead of re-downloading on every run. State
	// (vault/sessions/captures) stays fully isolated under the temp SCOUT_HOME.
	if src := defaultBrowsersCache(); src != "" {
		dst := filepath.Join(home, "browsers")
		logf("pre-seeding browser cache: %s -> %s", src, dst)
		if cpErr := copyTree(src, dst); cpErr != nil {
			logf("  pre-seed failed (will download CfT instead): %v", cpErr)
		} else {
			logf("  pre-seed ok")
		}
	}

	// Mandatory cleanup: uninstall host (global registry), close browser, rm temp.
	var br *scout.Browser
	cleanup := func() {
		fmt.Println("\n--- cleanup ---")
		out, uErr := r.scoutCmd("", "capture-host", "uninstall")
		if uErr != nil {
			fmt.Printf("cleanup: capture-host uninstall error: %v (%s)\n", uErr, strings.TrimSpace(out))
		} else {
			fmt.Printf("cleanup: %s", out)
		}
		// Verify no stray registry entry remains.
		if regExists() {
			fmt.Println("cleanup: WARNING registry key still present:", nativeHost)
		} else {
			fmt.Println("cleanup: registry key removed:", nativeHost)
		}
		if br != nil {
			_ = br.Close()
		}
		if rmErr := os.RemoveAll(home); rmErr != nil {
			fmt.Printf("cleanup: rm temp %s: %v\n", home, rmErr)
		} else {
			fmt.Println("cleanup: removed temp SCOUT_HOME")
		}
	}
	var cleanupOnce sync.Once
	doCleanup := func() { cleanupOnce.Do(cleanup) }
	defer doCleanup()

	// Global watchdog: if the whole run wedges (e.g. SW never spawns), run cleanup
	// and force-exit so we never leak a browser or a registry entry.
	go func() {
		time.Sleep(7 * time.Minute)
		fmt.Println("\nFATAL: global watchdog tripped (7m); forcing cleanup + exit")
		doCleanup()
		os.Exit(2)
	}()

	// --- Step 1/2: vault init + capture-key init (capture the pairing nonce) ---
	if out, err := r.scoutCmd("", "vault", "init"); err != nil {
		r.record("vault init", false, "err=%v out=%s", err, oneLine(out))
		return summarize(r)
	} else {
		r.record("vault init", true, "%s", oneLine(out))
	}

	keyOut, keyErr := r.scoutCmd("", "vault", "capture-key", "init")
	if keyErr != nil {
		r.record("vault capture-key init", false, "err=%v out=%s", keyErr, oneLine(keyOut))
		return summarize(r)
	}
	nonce := parseNonce(keyOut)
	if nonce == "" {
		r.record("vault capture-key init", false, "no pairing nonce in output: %s", oneLine(keyOut))
		return summarize(r)
	}
	r.record("vault capture-key init", true, "pairing nonce captured (len=%d)", len(nonce))

	// --- Step 3: synthetic local login test server ---
	srv := newTestServer()
	defer srv.Close()
	port := portOf(srv.URL)
	r.record("test server up", true, "%s (/login sets cookie+localStorage, / gates on both)", srv.URL)

	// --- Step 4: launch browser via the scout LIBRARY with the extension ---
	shippedExt, _ := filepath.Abs(filepath.Join("extensions", "scout-capture"))
	shippedManifest := filepath.Join(shippedExt, "manifest.json")
	if _, statErr := os.Stat(shippedManifest); statErr != nil {
		r.record("locate extension", false, "missing %s: %v", shippedExt, statErr)
		return summarize(r)
	}

	// PERMISSION GATE: the shipped manifest MUST declare "storage" — background.js/
	// popup.js rely on chrome.storage.local (pairingNonce + audit). Read it from disk
	// so the finding reflects what ships, regardless of any patched copy used
	// downstream. This was a prior release-blocker (storage omitted -> chrome.storage
	// undefined in the SW -> captureSession threw TypeError); it is now fixed.
	shippedPerms := manifestPermissions(shippedManifest)
	storageDeclared := slices.Contains(shippedPerms, "storage")
	r.record("shipped manifest declares \"storage\"", storageDeclared,
		"shipped permissions=%v", shippedPerms)
	if !storageDeclared {
		r.record("MANIFEST REGRESSION (shipped extension)", false,
			"extensions/scout-capture/manifest.json omits \"storage\"; chrome.storage is undefined in the SW "+
				"so captureSession cannot read pairingNonce -> shipped Scout Capture is NON-FUNCTIONAL")
		return summarize(r)
	}

	// Assert the SHIPPED (unmodified) extension actually exposes chrome.storage.local
	// in its service worker — i.e. the fixed manifest works with NO TypeError. This
	// loads the real shipped extension in its own short-lived browser (no CDP tab
	// gesture needed for a pure capability probe).
	shippedStorageOK, shippedProbe := assertShippedStorageAvailable(shippedExt, srv.URL)
	r.record("SHIPPED extension: chrome.storage.local available", shippedStorageOK,
		"%s (proves the fixed shipped manifest unblocks captureSession, no TypeError)", shippedProbe)
	if !shippedStorageOK {
		return summarize(r)
	}

	// To validate the REST of the pipeline (native-messaging host, spool seal,
	// import, replay) we launch against a TEMP COPY with the manifest fixed:
	// add "storage" (the bug), plus "tabs" + host_permissions to grant the
	// cookie/web-storage access that the real popup obtains via the activeTab
	// user gesture (which cannot be simulated over CDP). The wire contract,
	// background.js logic, and host are otherwise exercised unmodified.
	absExt, patchErr := makePatchedExtension(home, shippedExt)
	if patchErr != nil {
		r.record("prepare patched extension copy", false, "%v", patchErr)
		return summarize(r)
	}
	r.record("prepare patched extension copy", true,
		"temp copy with storage+tabs+host_permissions (mirrors popup activeTab grant): %s", absExt)

	br, cli, swSession, extID, mode, err := launchWithExtension(absExt, srv.URL)
	if err != nil {
		r.record("launch browser + reach SW", false, "%v", err)
		return summarize(r)
	}
	r.record("launch browser + reach SW", true, "mode=%s ext_id=%s", mode, extID)

	// Reuse the SAME CDP client used to discover the SW; flat-mode session IDs are
	// per-connection, so a second connection would invalidate swSession.
	sw := &swClient{cli: cli, session: swSession}

	// Capability probe on the PATCHED copy: confirms the temp copy has the access the
	// downstream native-messaging pipeline drives over CDP — chrome.storage.local
	// (also present in the shipped manifest, re-asserted above) PLUS tabs/host
	// access that a real popup gets via the activeTab user gesture (which cannot be
	// simulated over CDP). The shipped storage availability was already proven above.
	probe, _ := evalString(sw, `JSON.stringify({`+
		`perms:(chrome.runtime.getManifest().permissions||[]),`+
		`hasStorageLocal:(typeof chrome.storage!=="undefined"&&typeof chrome.storage.local!=="undefined"),`+
		`hasTabs:(typeof chrome.tabs!=="undefined")`+
		`})`)
	logf("SW capability probe (patched copy): %s", probe)
	storageOK := strings.Contains(probe, `"hasStorageLocal":true`) &&
		strings.Contains(probe, `"hasTabs":true`)
	r.record("patched copy: storage.local + tabs available", storageOK,
		"%s (CDP no-gesture path: storage + tabs/host access for captureSession)", probe)
	if !storageOK {
		return summarize(r)
	}

	// --- Login the active tab: navigate /login then / so cookie+storage exist ---
	// (Done inside launchWithExtension via a page; re-assert authentication here.)
	if authed, detail := assertTabAuthenticated(sw, port); !authed {
		r.record("tab logged in (pre-capture)", false, "%s", detail)
		return summarize(r)
	}
	r.record("tab logged in (pre-capture)", true, "page renders AUTHENTICATED")

	// --- Step 7: capture-host install <EXT_ID> ---
	if out, err := r.scoutCmd("", "capture-host", "install", extID); err != nil {
		r.record("capture-host install", false, "err=%v out=%s", err, oneLine(out))
		return summarize(r)
	} else {
		r.record("capture-host install", true, "%s", oneLine(out))
	}
	if !regExists() {
		r.record("HKCU registry entry present", false, "registry key missing after install")
		return summarize(r)
	}
	r.record("HKCU registry entry present", true, `HKCU\...\NativeMessagingHosts\%s`, nativeHost)

	spoolDir := filepath.Join(home, "captures", "spool")

	// --- Step 8: set nonce in SW + drive captureSession over native messaging ---
	beforeCount := countCaps(spoolDir)
	capRes, capErr := driveCaptureFresh(br, nonce, port)
	if capErr != nil {
		r.record("captureSession (native messaging)", false, "%v", capErr)
		// This is the make-or-break unknown. Dump diagnostics before giving up.
		dumpNativeMessagingDiagnostics(r, extID)
		return summarize(r)
	}
	if !capRes.OK {
		r.record("captureSession (native messaging)", false, "host rejected: ok=false error=%q", capRes.Error)
		dumpNativeMessagingDiagnostics(r, extID)
		return summarize(r)
	}
	r.record("captureSession (native messaging)", true, "ack id=%s — host spawned + sealed spool", capRes.ID)

	// --- Step 9: verify a NEW sealed spool file exists, 0600, not plaintext ---
	caps := listCaps(spoolDir)
	if len(caps) <= beforeCount || len(caps) == 0 {
		r.record("sealed spool written", false, "spool count before=%d after=%d in %s", beforeCount, len(caps), spoolDir)
		return summarize(r)
	}
	newCap := caps[len(caps)-1]
	if sealOK, detail := assertSealed(newCap); !sealOK {
		r.record("sealed spool written", false, "%s", detail)
		return summarize(r)
	} else {
		r.record("sealed spool written", true, "%s", detail)
	}

	// --- Step 10: import-captures (drains spool into per-site vault profile) ---
	impOut, impErr := r.scoutCmd("", "vault", "import-captures")
	if impErr != nil {
		r.record("vault import-captures", false, "err=%v out=%s", impErr, oneLine(impOut))
		return summarize(r)
	}
	if !strings.Contains(impOut, "imported 1 capture") && !strings.Contains(impOut, "imported") {
		r.record("vault import-captures", false, "unexpected: %s", oneLine(impOut))
		return summarize(r)
	}
	r.record("vault import-captures", true, "%s", oneLine(impOut))
	if remaining := countCaps(spoolDir); remaining != 0 {
		r.record("spool drained", false, "%d spool file(s) remain", remaining)
	} else {
		r.record("spool drained", true, "spool empty after import")
	}

	// --- Step 11: vault list -> find site profile; vault use --url -> replay ---
	listOut, listErr := r.scoutCmd("", "vault", "list")
	if listErr != nil {
		r.record("vault list", false, "err=%v out=%s", listErr, oneLine(listOut))
		return summarize(r)
	}
	profileID, host := parseProfileForSite(listOut, "127.0.0.1")
	if profileID == "" {
		r.record("vault list (find site profile)", false, "no profile for 127.0.0.1: %s", oneLine(listOut))
		return summarize(r)
	}
	r.record("vault list (find site profile)", true, "profile id=%s name~=%s", profileID, host)

	// Replay into a fresh isolated browser via `vault use --url`, then independently
	// verify the page rendered AUTHENTICATED (cookies + storage replayed).
	authed, replayDetail := replayAndAssertAuth(r, profileID, srv.URL)
	r.record("vault use replay -> AUTHENTICATED", authed, "%s", replayDetail)

	// --- Step 12a: NEGATIVE — wrong nonce => {ok:false}, no new spool ---
	beforeNeg := countCaps(spoolDir)
	negRes, negErr := driveCaptureFresh(br, "definitely-the-wrong-nonce", port)
	switch {
	case negErr != nil:
		// A transport error is also an acceptable rejection as long as no spool.
		r.record("negative: wrong nonce rejected", countCaps(spoolDir) == beforeNeg,
			"transport err=%v; spool unchanged=%v", negErr, countCaps(spoolDir) == beforeNeg)
	case negRes.OK:
		r.record("negative: wrong nonce rejected", false, "host ACCEPTED a wrong nonce (id=%s)", negRes.ID)
	default:
		noNewSpool := countCaps(spoolDir) == beforeNeg
		r.record("negative: wrong nonce rejected", noNewSpool,
			"ok=false error=%q; no new spool=%v", negRes.Error, noNewSpool)
	}

	// --- Step 12b: NEGATIVE — uninstall host => captureSession fails, no spool ---
	if out, err := r.scoutCmd("", "capture-host", "uninstall"); err != nil {
		r.record("capture-host uninstall (neg setup)", false, "err=%v out=%s", err, oneLine(out))
	} else {
		r.record("capture-host uninstall (neg setup)", true, "%s", oneLine(out))
	}
	beforeNeg2 := countCaps(spoolDir)
	neg2Res, neg2Err := driveCaptureFresh(br, nonce, port)
	switch {
	case neg2Err != nil:
		r.record("negative: host unreachable", countCaps(spoolDir) == beforeNeg2,
			"transport err=%v; spool unchanged=%v", neg2Err, countCaps(spoolDir) == beforeNeg2)
	case neg2Res.OK:
		r.record("negative: host unreachable", false, "captureSession SUCCEEDED after uninstall (id=%s)", neg2Res.ID)
	default:
		noNewSpool := countCaps(spoolDir) == beforeNeg2
		r.record("negative: host unreachable", noNewSpool,
			"ok=false error=%q; no new spool=%v", neg2Res.Error, noNewSpool)
	}

	return summarize(r)
}

// ---------- browser launch + service-worker discovery ----------

// launchWithExtension launches a browser via the scout library with the capture
// extension loaded, logs the active tab in at loginBaseURL, locates the
// extension's service-worker CDP target, and returns the SW session + ext id.
// It tries headless=new first; on SW/eval failure it retries headed.
func launchWithExtension(absExt, loginBaseURL string) (br *scout.Browser, cli *cdp.Client, swSession proto2.TargetSessionID, extID, mode string, err error) {
	for _, headless := range []bool{true, false} {
		mode = "headless=new"
		if !headless {
			mode = "headed"
		}
		logf("launchWithExtension: scout.New (%s) ext=%s", mode, absExt)
		b, lerr := scout.New(
			scout.WithExtension(absExt),
			scout.WithNoSandbox(),
			scout.WithHeadless(headless),
			scout.WithoutBridge(), // avoid bridge extension noise in target list
			// Reusable so sibling `scout.exe` invocations (which run
			// CleanStaleSessions on startup) do NOT reap this live browser:
			// the reaper preserves reusable sessions within their lifetime.
			scout.WithReusableSession(),
			scout.WithReusableLifetime(1*time.Hour),
		)
		if lerr != nil {
			err = fmt.Errorf("scout.New(%s): %w", mode, lerr)
			logf("  scout.New failed: %v", lerr)
			continue
		}
		logf("  scout.New ok; CDPURL=%s", truncate(b.CDPURL(), 70))

		// Log the tab in: /login (sets cookie + localStorage), then / (gated page).
		logf("  navigating /login")
		page, perr := b.NewPage(loginBaseURL + "/login")
		if perr == nil {
			perr = page.WaitLoad()
		}
		if perr == nil {
			logf("  navigating /")
			perr = page.Navigate(loginBaseURL + "/")
		}
		if perr == nil {
			perr = page.WaitLoad()
		}
		if perr != nil {
			_ = b.Close()
			err = fmt.Errorf("login navigation (%s): %w", mode, perr)
			logf("  navigation failed: %v", perr)
			continue
		}
		logf("  tab navigated; connecting CDP to locate service worker")

		// Find the extension service-worker target (it may take a moment to spawn).
		c, cerr := connectCDP(b.CDPURL())
		if cerr != nil {
			_ = b.Close()
			err = fmt.Errorf("cdp connect (%s): %w", mode, cerr)
			logf("  cdp connect failed: %v", cerr)
			continue
		}
		session, id, ferr := findExtensionSW(c)
		if ferr != nil {
			_ = b.Close()
			err = fmt.Errorf("find SW (%s): %w", mode, ferr)
			logf("  find SW failed (%s): %v", mode, ferr)
			continue
		}
		return b, c, session, id, mode, nil
	}
	return nil, nil, "", "", mode, err
}

// assertShippedStorageAvailable loads the SHIPPED (unmodified) extension in its own
// short-lived browser, locates its service worker, and probes whether
// chrome.storage.local is defined. With the fixed manifest ("storage" granted) this
// must be true — i.e. captureSession's chrome.storage.local access no longer throws a
// TypeError. The browser is always closed before returning. No CDP tab gesture is
// needed: this is a pure capability probe against the real shipped manifest.
func assertShippedStorageAvailable(shippedExt, loginBaseURL string) (bool, string) {
	for _, headless := range []bool{true, false} {
		mode := "headless=new"
		if !headless {
			mode = "headed"
		}
		logf("assertShippedStorageAvailable: scout.New (%s) ext=%s", mode, shippedExt)
		b, lerr := scout.New(
			scout.WithExtension(shippedExt),
			scout.WithNoSandbox(),
			scout.WithHeadless(headless),
			scout.WithoutBridge(),
		)
		if lerr != nil {
			logf("  scout.New failed: %v", lerr)
			continue
		}
		// Navigate a real page so the MV3 worker has reason to spawn.
		if page, perr := b.NewPage(loginBaseURL + "/"); perr == nil {
			_ = page.WaitLoad()
		}
		c, cerr := connectCDP(b.CDPURL())
		if cerr != nil {
			_ = b.Close()
			logf("  cdp connect failed: %v", cerr)
			continue
		}
		session, _, ferr := findExtensionSW(c)
		if ferr != nil {
			_ = b.Close()
			logf("  find SW failed (%s): %v", mode, ferr)
			continue
		}
		sw := &swClient{cli: c, session: session}
		probe, perr := evalString(sw, `JSON.stringify({`+
			`perms:(chrome.runtime.getManifest().permissions||[]),`+
			`hasStorage:(typeof chrome.storage!=="undefined"),`+
			`hasStorageLocal:(typeof chrome.storage!=="undefined"&&typeof chrome.storage.local!=="undefined")`+
			`})`)
		_ = b.Close()
		if perr != nil {
			logf("  storage probe eval failed: %v", perr)
			continue
		}
		logf("SHIPPED SW capability probe: %s", probe)
		ok := strings.Contains(probe, `"hasStorageLocal":true`)
		return ok, probe
	}
	return false, "could not reach shipped extension service worker"
}

// findExtensionSW polls Target.getTargets for a service_worker whose URL is a
// chrome-extension:// origin, attaches flat, and reads chrome.runtime.id.
// It also tries to wake a lazy MV3 worker: if it finds the extension via a
// background/other target it nudges discovery. Logs the target inventory.
func findExtensionSW(cli *cdp.Client) (proto2.TargetSessionID, string, error) {
	// Enable target discovery so service-worker targets are reported promptly.
	_, _ = cli.Call(context.Background(), "", "Target.setDiscoverTargets",
		map[string]any{"discover": true})

	deadline := time.Now().Add(45 * time.Second)
	var lastErr error
	logged := false
	for time.Now().Before(deadline) {
		res, err := proto2.TargetGetTargets{}.Call(cli)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if !logged {
			for _, t := range res.TargetInfos {
				logf("target: type=%-15s url=%s", t.Type, truncate(t.URL, 80))
			}
			logged = true
		}
		for _, t := range res.TargetInfos {
			if t.Type != proto2.TargetTargetInfoTypeServiceWorker {
				continue
			}
			if !strings.HasPrefix(t.URL, "chrome-extension://") {
				continue
			}
			att, aerr := proto2.TargetAttachToTarget{TargetID: t.TargetID, Flatten: true}.Call(cli)
			if aerr != nil {
				lastErr = aerr
				continue
			}
			sc := &swClient{cli: cli, session: att.SessionID}
			id, rerr := evalString(sc, "chrome.runtime.id")
			if rerr != nil || id == "" {
				lastErr = fmt.Errorf("read chrome.runtime.id: %w", rerr)
				continue
			}
			logf("found extension service_worker: id=%s", id)
			return att.SessionID, id, nil
		}
		// Re-print inventory periodically so a long wait is visible.
		logged = false
		time.Sleep(2 * time.Second)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no extension service_worker target appeared within deadline")
	}
	return "", "", lastErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ---------- CDP eval helpers (session-scoped proto.Client adapter) ----------

// connectCDP dials the browser CDP endpoint and starts draining the event
// channel. cdp.Client BLOCKS on its unbuffered Event() channel if events are not
// consumed (see internal/engine/lib/cdp/client.go) — Chrome emits events
// constantly, so without a drainer every Call() would hang forever.
func connectCDP(url string) (*cdp.Client, error) {
	cli, err := cdp.StartWithURL(context.Background(), url, nil)
	if err != nil {
		return nil, err
	}
	go func() {
		for range cli.Event() { //nolint:revive // intentional drain
		}
	}()
	return cli, nil
}

// swClient adapts *cdp.Client to proto.Client + proto.Sessionable so proto calls
// are routed to a specific (service-worker) session.
type swClient struct {
	cli     *cdp.Client
	session proto2.TargetSessionID
}

func (s *swClient) Call(ctx context.Context, sessionID, method string, params any) ([]byte, error) {
	// Force our session; ignore the (empty) sessionID proto would otherwise pass.
	return s.cli.Call(ctx, string(s.session), method, params)
}

func (s *swClient) GetSessionID() proto2.TargetSessionID { return s.session }

func evalRaw(c proto2.Client, expr string, awaitPromise bool) (*proto2.RuntimeEvaluateResult, error) {
	return proto2.RuntimeEvaluate{
		Expression:    expr,
		ReturnByValue: true,
		AwaitPromise:  awaitPromise,
	}.Call(c)
}

func evalString(c proto2.Client, expr string) (string, error) {
	res, err := evalRaw(c, expr, false)
	if err != nil {
		return "", err
	}
	if res.ExceptionDetails != nil {
		return "", fmt.Errorf("eval exception: %s", res.ExceptionDetails.Text)
	}
	if res.Result == nil {
		return "", nil
	}
	return res.Result.Value.Str(), nil
}

// ---------- capture driving (the native-messaging round trip) ----------

type captureResult struct {
	OK    bool   `json:"ok"`
	ID    string `json:"id"`
	Error string `json:"error"`
}

// freshSW opens a brand-new CDP connection to the browser, locates the extension
// service worker, attaches flat, and returns a session-scoped client. MV3 workers
// terminate when idle and CDP sessions are per-connection, so each capture attempt
// re-establishes a live session rather than reusing a possibly-dead one.
func freshSW(br *scout.Browser) (*swClient, error) {
	cli, err := connectCDP(br.CDPURL())
	if err != nil {
		return nil, fmt.Errorf("reconnect cdp: %w", err)
	}
	session, _, ferr := findExtensionSW(cli)
	if ferr != nil {
		return nil, fmt.Errorf("re-find SW: %w", ferr)
	}
	return &swClient{cli: cli, session: session}, nil
}

// driveCaptureFresh re-establishes a live SW session then drives one capture.
func driveCaptureFresh(br *scout.Browser, nonce, port string) (captureResult, error) {
	sw, err := freshSW(br)
	if err != nil {
		return captureResult{}, err
	}
	return driveCapture(sw, nonce, port)
}

// driveCapture sets pairingNonce in the SW's chrome.storage.local, then calls
// captureSession() on the test-site tab and returns the resolved {ok,id,error}.
func driveCapture(sw *swClient, nonce string, port string) (captureResult, error) {
	// Set the nonce (await the storage.set callback).
	setExpr := fmt.Sprintf(
		`new Promise(res=>chrome.storage.local.set({pairingNonce:%s},()=>res(true)))`,
		jsString(nonce))
	if _, err := evalRaw(sw, setExpr, true); err != nil {
		return captureResult{}, fmt.Errorf("set pairingNonce: %w", err)
	}

	// Locate the test-site tab. Without the "tabs" permission a {url:...} filter
	// returns nothing (url is stripped), so query ALL tabs and pick by URL, then
	// fall back to the active tab. Mirrors the popup's runtime "capture" command,
	// which uses {active:true,currentWindow:true}.
	wantHost := "127.0.0.1:" + port
	capExpr := fmt.Sprintf(`new Promise(res=>{`+
		`function pick(tabs){`+
		`  tabs=tabs||[];`+
		`  var t=tabs.find(x=>x.url&&x.url.indexOf(%s)>=0);`+
		`  if(!t)t=tabs.find(x=>x.active);`+
		`  if(!t)t=tabs[0];`+
		`  if(!t){res({ok:false,error:"no tab; n="+tabs.length});return;}`+
		`  res({__tab:{id:t.id,url:t.url,active:t.active,n:tabs.length}});`+
		`}`+
		`chrome.tabs.query({},pick);`+
		`})`,
		jsString(wantHost))
	// First resolve the tab object so we can log/diagnose, then capture it.
	tabRes, terr := evalRaw(sw, capExpr, true)
	if terr != nil {
		return captureResult{}, fmt.Errorf("tabs.query eval: %w", terr)
	}
	if tabRes.ExceptionDetails != nil {
		return captureResult{}, fmt.Errorf("tabs.query exception: %s", tabRes.ExceptionDetails.Text)
	}
	tabJSON := ""
	if tabRes.Result != nil {
		tabJSON = tabRes.Result.Value.JSON("", "")
	}
	logf("tabs.query -> %s", tabJSON)
	var probe struct {
		Tab *struct {
			ID     int    `json:"id"`
			URL    string `json:"url"`
			Active bool   `json:"active"`
			N      int    `json:"n"`
		} `json:"__tab"`
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.Unmarshal([]byte(tabJSON), &probe)
	if probe.Tab == nil {
		return captureResult{OK: false, Error: probe.Error}, nil
	}

	// Now drive captureSession on the picked tab id. captureSession re-queries the
	// tab object itself via the tab we pass; we pass a synthetic tab with id+url
	// (the SW reads tab.url + tab.id), matching what the popup hands it.
	capRun := fmt.Sprintf(`new Promise(res=>{`+
		`chrome.tabs.get(%d,function(t){`+
		`  var tab=(t&&t.url)?t:{id:%d,url:%s};`+
		`  captureSession(tab).then(res).catch(e=>res({ok:false,error:String(e)}));`+
		`});`+
		`})`,
		probe.Tab.ID, probe.Tab.ID, jsString("http://"+wantHost+"/"))
	capExpr = capRun

	res, err := evalRaw(sw, capExpr, true)
	if err != nil {
		return captureResult{}, fmt.Errorf("captureSession eval: %w", err)
	}
	if res.ExceptionDetails != nil {
		return captureResult{}, fmt.Errorf("captureSession exception: %s", res.ExceptionDetails.Text)
	}
	if res.Result == nil {
		return captureResult{}, fmt.Errorf("captureSession returned no value")
	}
	var cr captureResult
	if err := json.Unmarshal([]byte(res.Result.Value.JSON("", "")), &cr); err != nil {
		return captureResult{}, fmt.Errorf("decode captureSession result %q: %w", res.Result.Value.JSON("", ""), err)
	}
	return cr, nil
}

// ---------- authentication assertions ----------

// assertTabAuthenticated re-navigates the logged-in tab's origin in the SW's
// context is not possible; instead we read the gated marker by re-querying the
// tab via the service worker is also indirect. We use a direct CDP page eval.
// Here we attach to the page target and read document.body.innerText.
func assertTabAuthenticated(sw *swClient, port string) (bool, string) {
	// Find the page target for the test site and eval its body text.
	res, err := proto2.TargetGetTargets{}.Call(sw.cli)
	if err != nil {
		return false, fmt.Sprintf("getTargets: %v", err)
	}
	want := "127.0.0.1:" + port
	for _, t := range res.TargetInfos {
		if t.Type != proto2.TargetTargetInfoTypePage {
			continue
		}
		if !strings.Contains(t.URL, want) {
			continue
		}
		att, aerr := proto2.TargetAttachToTarget{TargetID: t.TargetID, Flatten: true}.Call(sw.cli)
		if aerr != nil {
			return false, fmt.Sprintf("attach page: %v", aerr)
		}
		pc := &swClient{cli: sw.cli, session: att.SessionID}
		txt, eerr := evalString(pc, "document.body && document.body.innerText")
		if eerr != nil {
			return false, fmt.Sprintf("eval body: %v", eerr)
		}
		if strings.Contains(txt, "AUTHENTICATED") {
			return true, "AUTHENTICATED"
		}
		return false, fmt.Sprintf("body=%q", strings.TrimSpace(txt))
	}
	return false, "no page target for test site"
}

// replayAndAssertAuth proves the captured profile authenticates the site:
//  1. A CONTROL clean browser (no injection) renders the gated page and must show
//     ANON, proving the marker is meaningful.
//  2. The documented `scout vault use <id> --url <base>/` path runs end to end.
//  3. An OBSERVABLE in-process replay opens the same vault, injects cookie + web
//     storage into a page we keep open (mirroring vault use's ApplyToPage +
//     ApplyStorageToPage), and reads the rendered AUTHENTICATED verdict directly.
func replayAndAssertAuth(r *runner, profileID, baseURL string) (bool, string) {
	// (1) Control: clean browser, no injection, against the IMMEDIATE-gating "/"
	// (computes the verdict at first paint). With no cookie+storage it must be ANON.
	control, berr := scout.New(scout.WithNoSandbox(), scout.WithHeadless(true), scout.WithoutBridge())
	if berr != nil {
		return false, fmt.Sprintf("control browser: %v", berr)
	}
	controlText := ""
	if page, perr := control.NewPage(baseURL + "/"); perr == nil {
		if perr = page.WaitLoad(); perr == nil {
			if res, _ := page.Eval("() => document.body.innerText"); res != nil {
				controlText = res.String()
			}
		}
	}
	_ = control.Close()
	if !strings.Contains(controlText, "ANON") {
		return false, fmt.Sprintf("control (no injection) did not render ANON: %q", marker(controlText))
	}

	// (2) Documented CLI replay path — proves `scout vault use` runs end to end.
	cliOut, cliErr := r.scoutCmd("", "vault", "use", profileID, "--url", baseURL+"/")
	cliMsg := oneLine(cliOut)
	if cliErr != nil {
		return false, fmt.Sprintf("control=ANON; cli `vault use` err=%v out=%s", cliErr, cliMsg)
	}

	// (3) OBSERVABLE in-process replay: open the same vault (public API, same
	// passphrase+path the CLI uses), inject cookie + web storage into a page WE
	// keep open, then read the rendered marker. This mirrors `vault use`'s exact
	// injection (ApplyToPage + ApplyStorageToPage) without the fire-and-forget
	// page close, so the AUTHENTICATED render is directly observed.
	verdict, cookieN, storeN, ierr := injectAndRead(r.home, profileID, baseURL)
	if ierr != nil {
		return false, fmt.Sprintf("control=ANON; cli=%q; in-process replay: %v", cliMsg, ierr)
	}
	ok := strings.Contains(verdict, "AUTHENTICATED")
	return ok, fmt.Sprintf("control=ANON; cli=%q; profile(cookies=%d,storageOrigins=%d); replayed-render=%s (expect AUTHENTICATED)",
		cliMsg, cookieN, storeN, marker(verdict))
}

// injectAndRead opens the vault in-process (same passphrase + path as the CLI),
// resolves the profile handle, and replays it into a page WE keep open: cookies
// via ApplyToPage (pre-nav), then navigate the gated page, then web storage via
// ApplyStorageToPage (post-nav, exactly as `vault use` does). It then recomputes
// the gate (cookie + localStorage) in-page and returns the verdict, so the
// AUTHENTICATED render is directly observed. cookieN/storeN are best-effort
// counts read back from the live page after injection.
func injectAndRead(home, profileID, baseURL string) (verdict string, cookieN, storeN int, err error) {
	vpath := filepath.Join(home, "profiles", "vault.bin")
	v, oerr := vault.Open([]byte(passphrase), vault.WithPath(vpath))
	if oerr != nil {
		return "", 0, 0, fmt.Errorf("vault.Open: %w", oerr)
	}
	defer func() { _ = v.Close() }()
	h, uerr := v.Use(profileID)
	if uerr != nil {
		return "", 0, 0, fmt.Errorf("vault.Use(%s): %w", profileID, uerr)
	}
	defer func() { _ = h.Close() }()

	b, berr := scout.New(scout.WithNoSandbox(), scout.WithHeadless(true), scout.WithoutBridge())
	if berr != nil {
		return "", 0, 0, fmt.Errorf("browser: %w", berr)
	}
	defer func() { _ = b.Close() }()

	page, perr := b.NewPage("about:blank")
	if perr != nil {
		return "", 0, 0, fmt.Errorf("new page: %w", perr)
	}
	// Cookies (and headers) BEFORE navigation, like vault use.
	if aerr := h.ApplyToPage(page); aerr != nil {
		return "", 0, 0, fmt.Errorf("ApplyToPage: %w", aerr)
	}
	if nerr := page.Navigate(baseURL + "/"); nerr != nil {
		return "", 0, 0, fmt.Errorf("navigate: %w", nerr)
	}
	if werr := page.WaitLoad(); werr != nil {
		return "", 0, 0, fmt.Errorf("waitload: %w", werr)
	}
	// Web storage AFTER navigation, like vault use.
	if aerr := h.ApplyStorageToPage(page); aerr != nil {
		return "", 0, 0, fmt.Errorf("ApplyStorageToPage: %w", aerr)
	}
	// Recompute the gate now that BOTH cookie and storage are present.
	res, eerr := page.Eval(`() => {` +
		`var hasCookie=document.cookie.indexOf('sid=')>=0;` +
		`var hasLS=localStorage.getItem('auth')==='yes';` +
		`var nCookie=document.cookie?document.cookie.split(';').filter(s=>s.trim()).length:0;` +
		`return JSON.stringify({v:(hasCookie&&hasLS)?'AUTHENTICATED':'ANON',c:nCookie,ls:localStorage.length});` +
		`}`)
	if eerr != nil {
		return "", 0, 0, fmt.Errorf("eval gate: %w", eerr)
	}
	var probe struct {
		V  string `json:"v"`
		C  int    `json:"c"`
		LS int    `json:"ls"`
	}
	_ = json.Unmarshal([]byte(res.String()), &probe)
	storeOrigins := 0
	if probe.LS > 0 {
		storeOrigins = 1
	}
	return probe.V, probe.C, storeOrigins, nil
}

// ---------- spool assertions ----------

func assertSealed(path string) (bool, string) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, fmt.Sprintf("stat: %v", err)
	}
	mode := fi.Mode().Perm()
	data, rerr := os.ReadFile(path) //nolint:gosec
	if rerr != nil {
		return false, fmt.Sprintf("read: %v", rerr)
	}
	// Not plaintext: the synthetic secret value must not appear in the sealed blob.
	plaintextLeak := strings.Contains(string(data), "secret123") ||
		strings.Contains(string(data), "AUTHENTICATED") ||
		strings.Contains(string(data), `"auth"`)
	if plaintextLeak {
		return false, "PLAINTEXT LEAK: secret found in spool file"
	}
	// On Windows perm bits are coarse; report them but only require non-plaintext.
	return true, fmt.Sprintf("%s sealed=%dB mode=%#o not-plaintext", filepath.Base(path), len(data), mode)
}

// ---------- diagnostics ----------

func dumpNativeMessagingDiagnostics(r *runner, extID string) {
	fmt.Println("\n--- native-messaging diagnostics ---")
	fmt.Println("registry key present:", regExists())
	base, _ := os.UserConfigDir()
	manifestPath := filepath.Join(base, "Scout", "NativeMessagingHosts", nativeHost+".json")
	if b, err := os.ReadFile(manifestPath); err == nil {
		fmt.Printf("manifest %s:\n%s\n", manifestPath, string(b))
	} else {
		fmt.Printf("manifest %s: %v\n", manifestPath, err)
	}
	fmt.Println("expected allowed_origin: chrome-extension://" + extID + "/")
	fmt.Println("ext_id file:", filepath.Join(r.home, "captures", "ext_id"))
	if b, err := os.ReadFile(filepath.Join(r.home, "captures", "ext_id")); err == nil {
		fmt.Println("ext_id persisted:", strings.TrimSpace(string(b)))
	}
}

// ---------- small utilities ----------

// newTestServer is the synthetic local login site:
//   - /login   sets cookie sid=secret123 (Path=/) and localStorage auth=yes.
//   - /        renders AUTHENTICATED iff cookie sid (server-visible) AND
//     localStorage auth=yes (client-visible) are both present, else ANON.
//
// The gate computes its verdict at first paint, so the replay verification reads
// it directly via CDP after injecting cookie + storage (see injectAndRead).
func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "secret123", Path: "/"})
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><body><div id="out">logging-in</div><script>` +
			`localStorage.setItem('auth','yes');` +
			`document.getElementById('out').textContent='logged-in';` +
			`</script></body></html>`))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		_, cookieErr := req.Cookie("sid")
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!doctype html><html><body><div id="out">checking</div><script>`+
			`var hasCookie=%v;var hasLS=localStorage.getItem('auth')==='yes';`+
			`document.getElementById('out').textContent=(hasCookie&&hasLS)?'AUTHENTICATED':'ANON';`+
			`</script></body></html>`, cookieErr == nil)
	})

	return httptest.NewServer(mux)
}

// manifestPermissions reads the "permissions" array from a manifest.json.
func manifestPermissions(path string) []string {
	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil
	}
	var m struct {
		Permissions []string `json:"permissions"`
	}
	_ = json.Unmarshal(b, &m)
	return m.Permissions
}

// makePatchedExtension copies the shipped extension into <home>/ext-patched and
// rewrites its manifest to add the "storage" permission (the shipped bug) plus
// "tabs" + host_permissions:["<all_urls>"] — the access a real popup obtains via
// the activeTab user gesture, which cannot be granted over CDP. All other files
// (background.js, snapshot.js, popup.js) are copied verbatim and exercised as-is.
func makePatchedExtension(home, shipped string) (string, error) {
	dst := filepath.Join(home, "ext-patched")
	if err := copyTree(shipped, dst); err != nil {
		return "", fmt.Errorf("copy extension: %w", err)
	}
	mp := filepath.Join(dst, "manifest.json")
	b, err := os.ReadFile(mp) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return "", fmt.Errorf("parse manifest: %w", err)
	}
	perms := map[string]bool{}
	if cur, ok := m["permissions"].([]any); ok {
		for _, p := range cur {
			if s, ok := p.(string); ok {
				perms[s] = true
			}
		}
	}
	for _, p := range []string{"storage", "tabs"} {
		perms[p] = true
	}
	list := make([]string, 0, len(perms))
	for p := range perms {
		list = append(list, p)
	}
	m["permissions"] = list
	m["host_permissions"] = []string{"<all_urls>"}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal patched manifest: %w", err)
	}
	if err := os.WriteFile(mp, out, 0o600); err != nil {
		return "", fmt.Errorf("write patched manifest: %w", err)
	}
	return dst, nil
}

// defaultBrowsersCache returns the platform-default Scout browsers cache dir if
// it exists and contains a chrome/ subdir, else "".
func defaultBrowsersCache() string {
	var base string
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		base = filepath.Join(v, "Scout", "browsers")
	}
	if base == "" {
		if h, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(h, ".scout", "browsers")
		}
	}
	if base == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(base, "chrome")); err != nil {
		return ""
	}
	return base
}

// copyTree recursively copies src to dst, preserving the directory layout.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, oerr := os.Open(p) //nolint:gosec
		if oerr != nil {
			return oerr
		}
		defer func() { _ = in.Close() }()
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
			return mkErr
		}
		out, cerr := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()) //nolint:gosec
		if cerr != nil {
			return cerr
		}
		if _, wErr := io.Copy(out, in); wErr != nil {
			_ = out.Close()
			return wErr
		}
		return out.Close()
	})
}

func scoutExePath() string {
	// Prefer the prebuilt repo-root scout.exe.
	if p, err := filepath.Abs("scout.exe"); err == nil {
		if _, statErr := os.Stat(p); statErr == nil {
			return p
		}
	}
	return "scout.exe"
}

func parseNonce(out string) string {
	for line := range strings.SplitSeq(out, "\n") {
		if _, after, found := strings.Cut(line, "pairing nonce:"); found {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// parseProfileForSite scans `vault list` output for a line mentioning host and
// returns the opaque id (first whitespace-separated token) + the matched line.
func parseProfileForSite(out, host string) (id, line string) {
	for ln := range strings.SplitSeq(out, "\n") {
		if strings.Contains(ln, host) {
			fields := strings.Fields(ln)
			if len(fields) > 0 {
				return fields[0], strings.TrimSpace(ln)
			}
		}
	}
	return "", ""
}

func listCaps(spoolDir string) []string {
	entries, err := os.ReadDir(spoolDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".cap" {
			out = append(out, filepath.Join(spoolDir, e.Name()))
		}
	}
	return out
}

func countCaps(spoolDir string) int { return len(listCaps(spoolDir)) }

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " | ")
	return s
}

func marker(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	return s
}

func portOf(u string) string {
	if i := strings.LastIndex(u, ":"); i >= 0 {
		return u[i+1:]
	}
	return ""
}

func jsString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err) // marshaling a string cannot fail
	}
	return string(b)
}

func summarize(r *runner) int {
	fmt.Println("\n================ SUMMARY ================")
	failed := 0
	for _, s := range r.steps {
		status := "PASS"
		if !s.pass {
			status = "FAIL"
			failed++
		}
		fmt.Printf("  [%s] %s\n", status, s.name)
	}
	fmt.Printf("=========================================\n")
	fmt.Printf("%d step(s), %d failed\n", len(r.steps), failed)
	if failed > 0 {
		return 1
	}
	return 0
}
