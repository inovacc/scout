// Command b3-investidor replays a saved B3 investor session headless and writes
// each section to raw JSON + flattened CSV. See README.md and the design spec.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/inovacc/scout/examples/b3-investidor/b3pipe"
)

func main() {
	profile := flag.String("profile", "", "vault profile ID for the B3 session (required)")
	sectionsPath := flag.String("sections", "sections.yaml", "path to sections.yaml")
	outRoot := flag.String("out", ".", "root directory for b3-data output")
	flag.Parse()

	if *profile == "" {
		fmt.Fprintln(os.Stderr, "error: --profile is required")
		os.Exit(2)
	}
	pass := []byte(os.Getenv("SCOUT_PASSPHRASE"))
	if len(pass) == 0 {
		fmt.Fprintln(os.Stderr, "error: set SCOUT_PASSPHRASE for the vault")
		os.Exit(2)
	}

	if err := run(*profile, pass, *sectionsPath, *outRoot); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(profile string, pass []byte, sectionsPath, outRoot string) error {
	cfg, err := b3pipe.LoadSections(sectionsPath)
	if err != nil {
		return err
	}
	browser, page, handle, err := b3pipe.OpenAuthedPage(profile, pass, cfg.BaseURL)
	if err != nil {
		return err
	}
	defer browser.Close()
	defer func() { _ = handle.Close() }()

	runOut, err := b3pipe.NewRun(outRoot, time.Now())
	if err != nil {
		return err
	}
	var results []b3pipe.SectionResult
	for _, s := range cfg.Sections {
		res, err := b3pipe.FetchSection(page, s, cfg.Auth)
		if err != nil {
			return err
		}
		if res.Status == 401 || res.Status == 0 {
			return fmt.Errorf("section %s returned %d — session expired; run `task b3:bootstrap` to re-login (gov.br + MFA)", s.ID, res.Status)
		}
		header, rows, err := b3pipe.Flatten(res.Body, s.RecordPath)
		if err != nil {
			// still write raw json for debugging; surface the flatten error
			_ = runOut.WriteSection(s.Output, res.Body, []string{"_raw"}, [][]string{{string(res.Body)}})
			return fmt.Errorf("section %s: %w", s.ID, err)
		}
		if err := runOut.WriteSection(s.Output, res.Body, header, rows); err != nil {
			return err
		}
		results = append(results, b3pipe.SectionResult{ID: s.ID, Status: res.Status, Rows: len(rows)})
		fmt.Printf("✓ %-16s %d  (%d rows)\n", s.ID, res.Status, len(rows))
	}
	if err := runOut.WriteManifest(b3pipe.Manifest{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Engine:    string(b3pipe.EngineB),
		Sections:  results,
	}); err != nil {
		return err
	}
	if err := runOut.UpdateLatest(); err != nil {
		return err
	}
	fmt.Printf("\nwrote %d sections to %s\n", len(results), runOut.Dir())
	return nil
}
