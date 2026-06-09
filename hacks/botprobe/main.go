// Command botprobe drives Scout (stealth, headless) against bot-detection and
// automation-practice sites to gauge the library's real-world maturity:
// does its stealth evade detectors, and can it navigate/extract reliably.
//
// Not part of the build (hacks/). Run: go run ./hacks/botprobe/ <url> [url...]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	urls := os.Args[1:]
	if len(urls) == 0 {
		fmt.Println("usage: go run ./hacks/botprobe/ <url> [url...]")
		os.Exit(2)
	}

	outDir := os.Getenv("BOTPROBE_OUT")
	if outDir == "" {
		outDir = "docs/quality/botprobe"
	}

	_ = os.MkdirAll(outDir, 0o755)

	b, err := scout.New(scout.WithStealth(), scout.WithHeadless(true), scout.WithNoSandbox())
	if err != nil {
		fmt.Println("LAUNCH ERROR:", err)
		os.Exit(1)
	}
	defer func() { _ = b.Close() }()

	// Stealth signals as the site's JS sees them. Eval requires function form.
	signals := []struct{ name, js string }{
		{"navigator.webdriver", `() => String(navigator.webdriver)`},
		{"window.chrome", `() => typeof window.chrome`},
		{"navigator.plugins", `() => navigator.plugins.length`},
		{"navigator.languages", `() => JSON.stringify(navigator.languages)`},
		{"hardwareConcurrency", `() => navigator.hardwareConcurrency`},
		{"userAgent", `() => navigator.userAgent`},
		{"UA-says-Headless", `() => /headless/i.test(navigator.userAgent)`},
		{"permissions(notif)", `() => (navigator.permissions ? 'present' : 'missing')`},
		{"webgl.vendor", `() => { try { const c=document.createElement('canvas').getContext('webgl'); const d=c.getExtension('WEBGL_debug_renderer_info'); return c.getParameter(d.UNMASKED_VENDOR_WEBGL); } catch(e){ return 'err:'+e.message; } }`},
	}

	for i, u := range urls {
		fmt.Printf("\n========== [%d] %s ==========\n", i+1, u)

		p, err := b.NewPage(u)
		if err != nil {
			fmt.Println("  NAVIGATE ERROR:", err)
			continue
		}

		_ = p.WaitLoad()
		time.Sleep(8 * time.Second) // let client-side detection finish

		if title, err := p.Title(); err == nil {
			fmt.Println("  title:", title)
		}

		fmt.Println("  -- automation signals the site can observe --")
		for _, s := range signals {
			if r, err := p.Eval(s.js); err == nil {
				v := strings.ReplaceAll(r.String(), "\n", " ")
				if len(v) > 120 {
					v = v[:120] + "…"
				}
				fmt.Printf("     %-20s = %s\n", s.name, v)
			} else {
				fmt.Printf("     %-20s = EVAL ERR: %v\n", s.name, err)
			}
		}

		if r, err := p.Eval(`() => (document.body && document.body.innerText || '').replace(/\n{2,}/g,'\n').slice(0, 1400)`); err == nil {
			fmt.Println("  -- visible page text (detector verdict) --")
			for _, ln := range strings.Split(strings.TrimSpace(r.String()), "\n") {
				if strings.TrimSpace(ln) != "" {
					fmt.Println("     |", strings.TrimSpace(ln))
				}
			}
		}

		shot := filepath.Join(outDir, fmt.Sprintf("site-%02d.png", i+1))
		if data, err := p.Screenshot(); err == nil {
			_ = os.WriteFile(shot, data, 0o644)
			fmt.Printf("  screenshot: %s (%d bytes)\n", shot, len(data))
		} else {
			fmt.Println("  screenshot error:", err)
		}

		_ = p.Close()
	}
}
