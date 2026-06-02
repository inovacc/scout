package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

// session classifications.
const (
	statusHealthy   = "HEALTHY"   // scout alive + browser alive
	statusReusable  = "REUSABLE"  // reusable flag set; respected per H6 (within ExpiresAt window)
	statusExpired   = "EXPIRED"   // reusable session past ExpiresAt — cleanable
	statusZombie    = "ZOMBIE"    // scout dead + browser alive (orphan browser)
	statusStale     = "STALE"     // scout dead + browser dead
	statusCorrupt   = "CORRUPT"   // scout.pid missing / unreadable
	statusForeign   = "FOREIGN"   // dir owned by different user (H4)
	statusUnknown   = "UNKNOWN"   // partial state
)

type auditEntry struct {
	ID               string    `json:"id"`
	Status           string    `json:"status"`
	Monitors         string    `json:"monitors,omitempty"`
	Reusable         bool      `json:"reusable"`
	ScoutPID         int       `json:"scout_pid"`
	ScoutAlive       bool      `json:"scout_alive"`
	BrowserPID       int       `json:"browser_pid"`
	BrowserAlive     bool      `json:"browser_alive"`
	BrowserParentPID int       `json:"browser_parent_pid,omitempty"`
	ParentMatchesScout bool    `json:"parent_matches_scout"`
	Browser          string    `json:"browser,omitempty"`
	Exec             string    `json:"exec,omitempty"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	Age              string    `json:"age,omitempty"`
	TTL              string    `json:"ttl,omitempty"`
	Err              string    `json:"err,omitempty"`
}

func init() {
	sessionCmd.AddCommand(sessionAuditCmd)
	sessionAuditCmd.Flags().Bool("kill", false, "kill zombie browsers and remove stale/corrupt dirs (does NOT touch healthy or reusable sessions)")
	sessionAuditCmd.Flags().Bool("json", false, "output as JSON instead of a table")
}

var sessionAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit all local sessions: classify each as HEALTHY / REUSABLE / ZOMBIE / STALE / CORRUPT and optionally kill zombies",
	Long: `Walks <scouthome>/sessions/* and classifies each session directory.

Classifications:
  HEALTHY   — scout and browser both alive, PIDs verify
  REUSABLE  — flagged reusable (preserved across daemon restart per H6)
  ZOMBIE    — scout process dead, browser still running (orphan to kill)
  STALE     — both scout and browser dead, dir can be removed
  CORRUPT   — scout.pid missing or unreadable
  FOREIGN   — dir owned by a different user (H4 rejection)

With --kill, zombies have their browser process killed and the dir
removed; stale and corrupt dirs are removed. Healthy and reusable
sessions are never touched.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		killFlag, _ := cmd.Flags().GetBool("kill")
		jsonFlag, _ := cmd.Flags().GetBool("json")

		entries, err := auditAllSessions()
		if err != nil {
			return err
		}

		if jsonFlag {
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		} else {
			printAuditTable(cmd, entries)
		}

		if killFlag {
			killed, removed := enforceAuditCleanup(entries)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nEnforce: killed %d zombie browsers, removed %d session dirs\n", killed, removed)
		}

		return nil
	},
}

// auditAllSessions classifies every session dir under SessionsDir.
func auditAllSessions() ([]auditEntry, error) {
	sessDir := scout.SessionsDir()

	dirEntries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("scout: read sessions dir: %w", err)
	}

	var out []auditEntry

	for _, d := range dirEntries {
		if !d.IsDir() {
			continue
		}

		out = append(out, classifySession(d.Name()))
	}

	return out, nil
}

func classifySession(id string) auditEntry {
	e := auditEntry{ID: id, Status: statusUnknown}

	info, err := scout.ReadSessionInfo(id)
	if err != nil {
		if os.IsNotExist(err) {
			e.Status = statusCorrupt
			e.Err = "no scout.pid"

			return e
		}
		// H4 ownership rejection presents here.
		e.Status = statusForeign
		e.Err = err.Error()

		return e
	}

	e.Reusable = info.Reusable
	e.ScoutPID = info.ScoutPID
	e.BrowserPID = info.BrowserPID
	e.BrowserParentPID = info.BrowserParentPID
	e.Browser = info.Browser
	e.Exec = info.Exec
	e.CreatedAt = info.CreatedAt
	e.ExpiresAt = info.ExpiresAt

	if !info.CreatedAt.IsZero() {
		e.Age = time.Since(info.CreatedAt).Truncate(time.Second).String()
	}

	if !info.ExpiresAt.IsZero() {
		ttl := time.Until(info.ExpiresAt).Truncate(time.Second)
		if ttl < 0 {
			e.TTL = "EXPIRED"
		} else {
			e.TTL = ttl.String()
		}
	}

	e.ScoutAlive = info.ScoutPID > 0 && scout.IsScoutProcess(info.ScoutPID)
	e.BrowserAlive = info.BrowserPID > 0 && scout.ProcessAlive(info.BrowserPID)

	// monitors.json sidecar — surface which monitors are active as a
	// short compact column (H=har, J=hijack, C=console, W=ws, Bn=N blocks).
	if cfg, _ := scout.ReadSessionMonitors(id); cfg != nil {
		var marks []byte
		if cfg.HAR.Enabled {
			marks = append(marks, 'H')
		}
		if cfg.Hijack.Enabled {
			marks = append(marks, 'J')
		}
		if cfg.Console.Enabled {
			marks = append(marks, 'C')
		}
		if cfg.WS.Enabled {
			marks = append(marks, 'W')
		}
		if n := len(cfg.Blocks); n > 0 {
			marks = append(marks, 'B')
			marks = append(marks, fmt.Sprintf("%d", n)...)
		}
		if len(marks) == 0 {
			marks = []byte("-")
		}
		e.Monitors = string(marks)
	}

	// Parent-pid sanity: a healthy session's browser PPID should equal the
	// scout PID that launched it.
	e.ParentMatchesScout = e.BrowserParentPID != 0 && e.BrowserParentPID == e.ScoutPID

	switch {
	case info.Reusable && info.IsExpired():
		e.Status = statusExpired
	case info.Reusable:
		e.Status = statusReusable
	case e.ScoutAlive && e.BrowserAlive:
		e.Status = statusHealthy
	case !e.ScoutAlive && e.BrowserAlive:
		e.Status = statusZombie
	case !e.ScoutAlive && !e.BrowserAlive:
		e.Status = statusStale
	}

	return e
}

func printAuditTable(cmd *cobra.Command, entries []auditEntry) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "SESSION\tSTATUS\tSCOUT\tBROWSER\tPPID\tBROWSER\tAGE\tTTL\tMON")
	_, _ = fmt.Fprintln(w, "-------\t------\t-----\t-------\t----\t-------\t---\t---\t---")

	for _, e := range entries {
		scoutCol := fmt.Sprintf("%d", e.ScoutPID)
		if !e.ScoutAlive && e.ScoutPID > 0 {
			scoutCol += "(†)"
		} else if e.ScoutAlive {
			scoutCol += "(✓)"
		}

		browserCol := fmt.Sprintf("%d", e.BrowserPID)
		if !e.BrowserAlive && e.BrowserPID > 0 {
			browserCol += "(†)"
		} else if e.BrowserAlive {
			browserCol += "(✓)"
		}

		ppidCol := fmt.Sprintf("%d", e.BrowserParentPID)
		if e.ParentMatchesScout {
			ppidCol += "(=scout)"
		} else if e.BrowserParentPID != 0 && e.ScoutPID != 0 {
			ppidCol += "(!=scout)"
		}

		shortID := e.ID
		if len(shortID) > 36 {
			shortID = shortID[:36]
		}

		mon := e.Monitors
		if mon == "" {
			mon = "-"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			shortID, e.Status, scoutCol, browserCol, ppidCol, e.Browser, e.Age, e.TTL, mon)
	}
}

// enforceAuditCleanup kills zombie browsers and removes stale/corrupt/expired
// dirs. Healthy and reusable sessions are NEVER touched.
//
// All non-healthy statuses route through scout.ReapSession which:
//   - kills the recorded BrowserPID (identity-verified, self-pid-guarded), AND
//   - kills any holder found via path-bounded FindBrowsersUsingDataDir scan —
//     the only path that reaches a zombie whose scout.pid is missing/corrupt.
//   - removes the session dir with retry on Windows lock contention.
//
// The former bare os.FindProcess(e.BrowserPID).Kill() block has been removed:
// it was PID-reuse-unsafe (no identity check), had no self-pid guard, and
// silently skipped the kill when scout.pid was corrupt (BrowserPID == 0).
func enforceAuditCleanup(entries []auditEntry) (killed, removed int) {
	for _, e := range entries {
		switch e.Status {
		case statusZombie, statusStale, statusCorrupt, statusExpired:
			stats := scout.ReapSession(e.ID)
			killed += stats.Killed
			removed += stats.Removed
		}
	}

	return killed, removed
}
