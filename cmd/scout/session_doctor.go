package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func init() {
	cmd := newSessionDoctorCmd()
	cmd.Flags().Bool("fix", false, "kill zombie browsers and remove stale/corrupt dirs, then re-audit")
	sessionCmd.AddCommand(cmd)
}

// newSessionDoctorCmd builds the `scout session doctor` command. It is a
// constructor (not a package var) so tests can build a fresh instance with
// isolated flags. REUSES auditAllSessions() + classifySession() from
// session_audit.go — no classification logic is duplicated here.
func newSessionDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Verify the session invariant: every folder owned by a live verified scout; no orphan browsers",
		Long: `Enumerates <scouthome>/sessions/* and asserts the hardening invariant:

  At rest (no live scout process), the sessions directory is empty. A folder
  may exist ONLY while a live, identity-verified scout process owns it, and
  every scout-launched browser must have a live scout parent.

Prints a per-folder scout(parent)/browser(child) PID mapping and a final
verdict. Exits non-zero on any violation (orphan browser / ownerless folder).

With --fix, kills zombie browsers and removes stale/corrupt/expired dirs via
the same enforcement path as 'scout session audit --kill', then re-audits and
re-evaluates the verdict.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := auditAllSessions()
			if err != nil {
				return err
			}

			fix, _ := cmd.Flags().GetBool("fix")
			if fix {
				killed, removed := enforceAuditCleanup(entries)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"Fix: killed %d zombie browser(s), removed %d dir(s); re-auditing...\n",
					killed, removed)

				entries, err = auditAllSessions()
				if err != nil {
					return err
				}
			}

			printDoctorMapping(cmd, entries)

			violations := doctorViolations(entries)
			if len(violations) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"\nVERDICT: invariant holds — %d folder(s), no orphan browsers, no ownerless folders\n",
					len(entries))

				return nil
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "\nVERDICT: invariant VIOLATED (%d):\n", len(violations))
			for _, v := range violations {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", v)
			}

			return fmt.Errorf("scout: session doctor: %d invariant violation(s)", len(violations))
		},
	}
}

// doctorViolations returns a human-readable description for every entry that
// breaks the §2 invariant: a folder may exist ONLY while a live, verified
// scout process owns it.
//
// Violations:
//   - ZOMBIE: scout dead but browser still alive (orphan browser)
//   - CORRUPT: scout.pid missing or unreadable — ownerless folder, invariant broken
//   - STALE or CORRUPT with a live browser: ownerless folder + live browser process
//
// Healthy, reusable, and expired-but-browser-dead folders are NOT violations
// (the reaper handles expiry).
func doctorViolations(entries []auditEntry) []string {
	var out []string

	for _, e := range entries {
		switch {
		case e.Status == statusZombie:
			out = append(out, fmt.Sprintf("%s: ZOMBIE (scout %d dead, browser %d ALIVE)",
				e.ID, e.ScoutPID, e.BrowserPID))
		case e.Status == statusCorrupt:
			out = append(out, fmt.Sprintf("%s: CORRUPT (ownerless folder — no valid scout.pid)",
				e.ID))
		case e.Status == statusStale && e.BrowserAlive:
			out = append(out, fmt.Sprintf("%s: STALE with live browser %d (ownerless + live browser)",
				e.ID, e.BrowserPID))
		}
	}

	return out
}

// printDoctorMapping prints the parent(scout)→child(browser) PID mapping for
// every folder so an operator can see exactly which process owns what.
func printDoctorMapping(cmd *cobra.Command, entries []auditEntry) {
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No session folders.")
		return
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "SESSION\tSTATUS\tSCOUT(parent)\tBROWSER(child)\tPARENT==SCOUT")
	_, _ = fmt.Fprintln(w, "-------\t------\t-------------\t--------------\t------------")

	for _, e := range entries {
		scoutCol := fmt.Sprintf("%d", e.ScoutPID)
		if e.ScoutAlive {
			scoutCol += "(alive)"
		} else if e.ScoutPID > 0 {
			scoutCol += "(dead)"
		}

		browserCol := fmt.Sprintf("%d", e.BrowserPID)
		if e.BrowserAlive {
			browserCol += "(alive)"
		} else if e.BrowserPID > 0 {
			browserCol += "(dead)"
		}

		shortID := e.ID
		if len(shortID) > 36 {
			shortID = shortID[:36]
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n",
			shortID, e.Status, scoutCol, browserCol, e.ParentMatchesScout)
	}
}
